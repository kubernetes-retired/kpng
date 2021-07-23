package ipvssink

import (
	"strconv"

	"sigs.k8s.io/kpng/pkg/api/localnetv1"
)

const (
	ClusterIPService    = "ClusterIP"
	NodePortService     = "NodePort"
	LoadBalancerService = "LoadBalancer"
)

func asDummyIPs(set *localnetv1.IPSet) (ips []string) {
	ips = make([]string, 0, len(set.V4)+len(set.V6))

	for _, ip := range set.V4 {
		ips = append(ips, ip+"/32")
	}
	for _, ip := range set.V6 {
		ips = append(ips, ip+"/128")
	}

	return
}

func epPortSuffix(port *localnetv1.PortMapping) string {
	return port.Protocol.String() + ":" + strconv.Itoa(int(port.Port))
}

// diffInPortMapping TODO, we should support this logic in the diffstore, this is a temporary workaround.
func diffInPortMapping(previous, current *localnetv1.Service) (added, removed []*localnetv1.PortMapping) {
	for _, p1 := range previous.Ports {
		found := false
		for _, p2 := range current.Ports {
			if p1.Name == p2.Name && p1.Port == p2.Port && p1.Protocol == p2.Protocol {
				found = true
				break
			}
		}

		if !found {
			removed = append(removed, p1)
		}
	}

	for _, p1 := range current.Ports {
		found := false
		for _, p2 := range previous.Ports {
			if p1.Name == p2.Name && p1.Port == p2.Port && p1.Protocol == p2.Protocol {
				found = true
				break
			}
		}

		if !found {
			added = append(added, p1)
		}
	}
	return
}
