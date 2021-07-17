package ipvssink

import (
	"strconv"

	"sigs.k8s.io/kpng/pkg/api/localnetv1"
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
