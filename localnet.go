package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/OneOfOne/xxhash"
	"github.com/golang/protobuf/proto"

	"k8s.io/kube-localnet-api/pkg/api/localnetv1"
	"k8s.io/kube-localnet-api/pkg/store"
)

var (
	netns          = flag.String("netns", "", "network namespace to use")
	iptChainPrefix = flag.String("iptables-chain-prefix", "k8s-", "prefix for iptables chains")
	extLBsOnly     = flag.Bool("external-lbs-only", false, "Only consider services of type LoadBalancer for external traffic")

	// computed service endpoints store
	sepStore = store.New()
)

func updateLocalnetAPI(change serviceEndpointsChange) {
	key := []byte(path.Join(change.Namespace, change.Name))

	svc := change.Service
	eps := change.Endpoints

	if svc == nil || eps == nil {
		sepStore.Set(key, nil)
		return
	}

	// compute external IPs
	extIPs := make([]string, 0)
	if lbIP := svc.Spec.LoadBalancerIP; lbIP != "" {
		extIPs = append(extIPs, lbIP)
	}

	if ingressStatus := svc.Status.LoadBalancer.Ingress; len(ingressStatus) != 0 {
		for _, ing := range ingressStatus {
			if ing.IP != "" {
				extIPs = append(extIPs, ing.IP)
			}
		}
	}

	if len(svc.Spec.ExternalIPs) != 0 {
		extIPs = append(extIPs, svc.Spec.ExternalIPs...)
	}

	// compute endpoints IPs
	epIPs := make([]string, 0, len(eps.Subsets))
	for _, subset := range eps.Subsets {
		for _, addr := range subset.Addresses {
			epIPs = append(epIPs, addr.IP)
		}
	}

	sort.Strings(epIPs) // stable order

	// compute ports
	ports := make([]*localnetv1.PortMapping, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		ports = append(ports, &localnetv1.PortMapping{
			Name:       port.Name,
			Protocol:   string(port.Protocol),
			Port:       port.Port,
			NodePort:   port.NodePort,
			TargetPort: int32(port.TargetPort.IntValue()), // TODO support string values
		})
	}

	// build the new value
	newValue := &localnetv1.ServiceEndpoints{
		Namespace: change.Namespace,
		Name:      change.Name,
		Type:      string(svc.Spec.Type),
		IPs: &localnetv1.ServiceIPs{
			ClusterIP:   svc.Spec.ClusterIP,
			ExternalIPs: extIPs,
			EndpointIPs: epIPs,
		},
		Ports: ports,
	}

	sepStore.Set(key, newValue)
}

func localnetExtIptables() {
	forwardChain := *iptChainPrefix + "forward"
	dnatChain := *iptChainPrefix + "DNAT"
	snatChain := *iptChainPrefix + "SNAT"

	var (
		rev      uint64
		prevHash uint64
	)

	for {
		snap := sepStore.Next(rev)
		rev = snap.Rev()

		log.Print("ext-iptables: at rev ", rev)

		seps := make(SEps, 0)

		for kv := range snap.Iterate(func() proto.Message { return &localnetv1.ServiceEndpoints{} }) {
			if kv.Err != nil {
				log.Fatal("failed to iterate snapshot: ", kv.Err)
			}

			sep := kv.Value.(*localnetv1.ServiceEndpoints)

			if *extLBsOnly && sep.Type != "LoadBalancer" {
				// only process LBs
				continue
			}

			if len(sep.IPs.ExternalIPs) == 0 {
				// filter out services without external IPs
				continue
			}

			seps = append(seps, sep)
		}

		ipt := &bytes.Buffer{}

		fmt.Fprint(ipt, "*filter\n")
		fmt.Fprint(ipt, ":", forwardChain, " -\n")
		for _, sep := range seps {
			key := path.Join(sep.Namespace, sep.Name)
			for _, ip := range sep.IPs.EndpointIPs {
				for _, port := range sep.Ports {
					proto := strings.ToLower(port.Protocol)

					fmt.Fprintf(ipt, "-A %s -d %s -j ACCEPT -m %s -p %s --dport %d %s\n",
						forwardChain, ip, proto, proto, port.TargetPort,
						iptCommentf("%s: %s:%d -> %d", key, proto, port.Port, port.TargetPort))
				}
			}
		}

		fmt.Fprint(ipt, "COMMIT\n")

		// NAT chain
		fmt.Fprint(ipt, "*nat\n")
		fmt.Fprint(ipt, ":", dnatChain, " -\n")
		fmt.Fprint(ipt, ":", snatChain, " -\n")

		// DNAT rules
		for _, sep := range seps {
			key := path.Join(sep.Namespace, sep.Name)

			for _, extIP := range sep.IPs.ExternalIPs {
				epCount := len(sep.IPs.EndpointIPs)

				for i, ip := range sep.IPs.EndpointIPs {
					rndProba := iptRandom(i, epCount)

					for _, port := range sep.Ports {
						proto := strings.ToLower(port.Protocol)
						fmt.Fprintf(ipt, "-A %s -d %s -m %s -p %s --dport %d -j DNAT --to-destination %s:%d %s %s\n",
							dnatChain, extIP, proto, proto, port.Port, ip, port.TargetPort, rndProba,
							iptCommentf("%s: %s:%d -> %s:%d", key, extIP, port.Port, ip, port.TargetPort))
					}
				}
			}
		}

		// SNAT rules
		revExt := map[string]string{}
		revExtKey := map[string]string{}
		for _, sep := range seps {
			key := path.Join(sep.Namespace, sep.Name)

			// use the first external IP
			extIP := sep.IPs.ExternalIPs[0]

			for _, ip := range sep.IPs.EndpointIPs {
				if revExt[ip] == "" || extIP < revExt[ip] {
					revExt[ip] = extIP
					revExtKey[ip] = key
				}
			}
		}

		epIPs := make([]string, 0, len(revExt))
		for epIP := range revExt {
			epIPs = append(epIPs, epIP)
		}

		sort.Strings(epIPs)

		for _, epIP := range epIPs {
			extIP := revExt[epIP]
			fmt.Fprintf(ipt, "-A %s -s %s -j SNAT --to-source %s %s\n",
				snatChain, epIP, extIP,
				iptCommentf("%s: external IP", revExtKey[epIP]))
		}

		fmt.Fprint(ipt, "COMMIT\n")

		newHash := xxhash.Checksum64(ipt.Bytes())
		if prevHash == newHash {
			continue
		}

		log.Print("ext-iptables: rules have changed, updating")
		rules := ipt.Bytes()

		// setup iptables command
		var cmd *exec.Cmd
		if *netns == "" {
			cmd = exec.Command("iptables-restore", "--noflush")
		} else {
			cmd = exec.Command("ip", "netns", "exec", *netns, "iptables-restore", "--noflush")
		}

		cmd.Stdin = ipt
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			log.Print("ext-iptables: failed to restore iptables rules: ", err, "\n", string(rules))
			continue
		}

		prevHash = newHash
	}
}

func iptComment(comment string) string {
	return fmt.Sprintf("-m comment --comment %q", comment)
}

func iptCommentf(pattern string, values ...interface{}) string {
	return iptComment(fmt.Sprintf(pattern, values...))
}

func iptRandom(idx, count int) string {
	if count == 1 || idx+1 == count {
		return ""
	}
	return fmt.Sprintf(" -m statistic --mode random --probability %.4f", float64(idx+1)/float64(count))
}

type SEps []*localnetv1.ServiceEndpoints

func (a SEps) ExternalIPs() []string {
	ips := make(uniq, 0)

	for _, sep := range a {
		for _, ip := range sep.IPs.ExternalIPs {
			ips.Add(ip)
		}
	}

	return ips
}
