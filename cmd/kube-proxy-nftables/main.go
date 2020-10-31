package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/client"
)

var (
	dryRun       = flag.Bool("dry-run", false, "dry run (do not apply rules)")
	debug        = flag.Bool("debug", false, "debug nft script")
	hookPrio     = flag.Int("hook-priority", 0, "nftable hooks priority")
	skipComments = flag.Bool("skip-comments", false, "don't comment rules")
	splitBits    = flag.Int("split-bits", 24, "dispatch services in multiple chains, spliting at the nth bit")
	// TODO splitBits6   = flag.Int("split-bits6", 120, "dispatch services in multiple chains, spliting at the nth bit (for IPv6)")

	firstRun = true
)

func main() {
	client.Run(updateNftables)
}

func updateNftables(items []*localnetv1.ServiceEndpoints) {
	defer chainBuffers.Reset()

	rule := new(bytes.Buffer)

	ipv4Mask := net.CIDRMask(*splitBits, 32)
	// TODO ipv6Mask := net.CIDRMask(*splitBits6, 128)

	chainNets := map[string]*net.IPNet{}

	for _, endpoints := range items {
		// TODO doing IPv4 only for now

		// only handle cluster IPs
		if endpoints.Type != "ClusterIP" {
			continue
		}

		ips := make([]string, 0)
		if ip := endpoints.IPs.ClusterIP; ip != "" && ip != "None" {
			ips = append(ips, ip)
		}

		ips = append(ips, endpoints.IPs.ExternalIPs...)

		if len(ips) == 0 {
			continue
		}

		for _, m := range []struct {
			match    string
			protocol localnetv1.Protocol
		}{
			{"tcp dport", localnetv1.Protocol_TCP},
			{"udp dport", localnetv1.Protocol_UDP},
			{"sctp dport", localnetv1.Protocol_SCTP},
		} {
			rule.Reset()

			srcPorts := make([]string, 0)
			portMaps := make([]string, 0)
			var dstPort int32     // for the single port case
			portsIdentity := true // if every source port is mapped to the same target
			for _, port := range endpoints.Ports {
				if port.Protocol != m.protocol {
					continue
				}

				if portsIdentity && port.Port != port.TargetPort {
					portsIdentity = false
				}

				srcPorts = append(srcPorts, fmt.Sprintf("%d", port.Port))
				dstPort = port.TargetPort
				portMaps = append(portMaps, fmt.Sprintf("%d : %d", port.Port, port.TargetPort))
			}

			if len(srcPorts) == 0 {
				continue
			}

			if len(ips) == 1 {
				fmt.Fprintf(rule, "  ip daddr %s", ips[0])
			} else {
				fmt.Fprintf(rule, "  ip daddr {%s}", strings.Join(ips, ", "))
			}

			if len(srcPorts) == 1 {
				fmt.Fprintf(rule, " %s %s", m.match, srcPorts[0])
			} else {
				fmt.Fprintf(rule, " %s {%s}", m.match, strings.Join(srcPorts, ", "))
			}

			dstIPs := make([]string, 0, len(endpoints.Endpoints))
			dstMap := make([]string, 0, len(endpoints.Endpoints))
			for idx, ep := range endpoints.Endpoints {
				if len(ep.IPsV4) == 0 {
					continue
				}

				dstIPs = append(dstIPs, ep.IPsV4[0])
				dstMap = append(dstMap, fmt.Sprintf("%d : %s", idx, ep.IPsV4[0]))
			}

			//fmt.Fprintf(out, "comment \"dnat for %s/%s port %d\" ", endpoints.Namespace, endpoints.Name, port.Port)

			fmt.Fprint(rule, " ")
			if len(dstIPs) == 0 {
				fmt.Fprint(rule, "reject")
			} else {
				if len(dstIPs) == 1 {
					fmt.Fprintf(rule, "dnat to %s", dstIPs[0])
				} else {
					fmt.Fprintf(rule, "dnat to numgen random mod %d map {%s}", len(dstMap), strings.Join(dstMap, ", "))
				}

				if !portsIdentity {
					if len(portMaps) == 1 {
						fmt.Fprintf(rule, ":%d", dstPort)
					} else {
						fmt.Fprintf(rule, ":%s map { %s }", m.match, strings.Join(portMaps, ", "))
					}
				}
			}

			if !*skipComments {
				fmt.Fprintf(rule, " comment \"%s/%s %v ports\"", endpoints.Namespace, endpoints.Name, m.protocol)
			}

			fmt.Fprintln(rule)

			// filter or nat? reject does not work in prerouting
			prefix := "dnat_"
			if len(dstIPs) == 0 {
				prefix = "filter_"
			}

			ip := net.ParseIP(endpoints.IPs.ClusterIP).Mask(ipv4Mask)

			chain := prefix + hex.EncodeToString(ip)
			chainNets[chain] = &net.IPNet{
				IP:   ip,
				Mask: ipv4Mask,
			}

			rule.WriteTo(chainBuffers.Get(chain))

			if extIPs := endpoints.IPs.ExternalIPs; len(extIPs) != 0 {
				fmt.Fprintf(chainBuffers.Get(prefix+"external"), "  ip daddr {%s} goto %s\n", strings.Join(extIPs, ", "), chain)
			}
		}
	}

	// dispatch chain
	chains := chainBuffers.List()
	for _, prefix := range []string{"dnat_", "filter_"} {
		chain := chainBuffers.Get(prefix + "z_all")

		others := make([]string, 0)
		targets := make([]string, 0)
		for _, target := range chains {
			if !strings.HasPrefix(target, prefix) {
				continue
			}

			// z chains are excluded from auto-grab
			if strings.HasPrefix(target, prefix+"z_") {
				continue
			}

			ipNet := chainNets[target]
			if ipNet == nil {
				others = append(others, target)
				continue
			}

			ones, _ := ipNet.Mask.Size()
			targets = append(targets, fmt.Sprintf("%s/%d: jump %s", ipNet.IP.String(), ones, target))
		}

		if len(targets) != 0 {
			fmt.Fprintf(chain, "  ip daddr vmap { \\\n    %s}\n", strings.Join(targets, ", \\\n    "))
		}

		for _, other := range others {
			fmt.Fprintf(chain, "  goto %s\n", other)
		}
	}

	fmt.Fprintf(chainBuffers.Get("nat_z_forward"),
		"  type nat hook prerouting priority %d;\n  jump dnat_z_all\n", *hookPrio)
	fmt.Fprintf(chainBuffers.Get("nat_z_output"),
		"  type nat hook output priority %d;\n  jump dnat_z_all\n", *hookPrio)
	fmt.Fprintf(chainBuffers.Get("filter_z_forward"),
		"  type filter hook forward priority %d;\n  jump filter_z_all\n", *hookPrio)
	fmt.Fprintf(chainBuffers.Get("filter_z_output"),
		"  type filter hook output priority %d;\n  jump filter_z_all\n", *hookPrio)

	if !firstRun && !chainBuffers.Changed() {
		log.Print("no changes to apply")
		return
	}

	// render the rule set
	cmdIn, pipeOut := io.Pipe()

	go func() {
		outputs := make([]io.Writer, 0, 2)
		outputs = append(outputs, pipeOut)

		if *debug {
			outputs = append(outputs, os.Stdout)
		}

		out := bufio.NewWriter(io.MultiWriter(outputs...))

		chains := chainBuffers.List()
		if firstRun {
			fmt.Fprintln(out, "table ip k8s_svc")
			fmt.Fprintln(out, "flush table k8s_svc")

		} else {
			// update only changed rules
			changedChains := make([]string, 0, len(chains))

			// flush changed chains
			for _, chain := range chains {
				c := chainBuffers.Get(chain)
				if !c.Changed() {
					continue
				}

				if !c.Created() {
					fmt.Fprintf(out, "flush chain k8s_svc %s\n", chain)
				}

				changedChains = append(changedChains, chain)
			}

			chains = changedChains
		}

		// create/update changed chains
		if len(chains) != 0 {
			fmt.Fprintln(out, "table ip k8s_svc {")

			for _, chain := range chains {
				c := chainBuffers.Get(chain)

				fmt.Fprintf(out, " chain %s {\n", chain)
				io.Copy(out, c)
				fmt.Fprintln(out, " }")
			}

			fmt.Fprintln(out, "}")
		}

		// delete removed chains
		for _, chain := range chainBuffers.Deleted() {
			fmt.Fprintf(out, "delete chain k8s_svc %s\n", chain)
		}

		out.Flush()
		pipeOut.Close()
	}()

	if *dryRun {
		io.Copy(ioutil.Discard, cmdIn)
		log.Print("not running nft (dry run mode)")
	} else {
		cmd := exec.Command("nft", "-f", "-")
		cmd.Stdin = cmdIn
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		start := time.Now()
		err := cmd.Run()
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("nft failed: %v (%s)", err, elapsed)

			if !firstRun {
				// rewrite everything
				firstRun = true
				updateNftables(items)
			}

		} else {
			log.Printf("nft ok (%s)", elapsed)
		}
	}

	if firstRun {
		firstRun = false
	}
}
