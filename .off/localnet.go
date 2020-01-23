package main

import (
	"flag"
	"path"
	"sort"

	"github.com/mcluseau/kube-localnet-api/pkg/api/localnetv1"
	"github.com/mcluseau/kube-localnet-api/pkg/store"
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
			Protocol:   localnetv1.ParseProtocol(string(port.Protocol)),
			Port:       port.Port,
			NodePort:   port.NodePort,
			TargetPort: int32(port.TargetPort.IntValue()), // TODO support string values?
		})
	}

	// build the new value
	newValue := &localnetv1.ServiceEndpoints{
		Namespace: change.Namespace,
		Name:      change.Name,
		Type:      string(svc.Spec.Type),
		IPs: &localnetv1.ServiceIPs{
			ClusterIP:   svc.Spec.ClusterIP,
			ExternalIPs: &localnetv1.IPList{Items: extIPs},
			EndpointIPs: &localnetv1.IPList{Items: epIPs},
		},
		Ports: ports,
	}

	sepStore.Set(key, newValue)
}

type SEps []*localnetv1.ServiceEndpoints

func (a SEps) ExternalIPs() []string {
	count := 0
	for _, sep := range a {
		count += len(sep.IPs.ExternalIPs.Items)
	}

	ips := make(uniq, 0, count)

	for _, sep := range a {
		for _, ip := range sep.IPs.ExternalIPs.Items {
			ips.Add(ip)
		}
	}

	return ips
}
