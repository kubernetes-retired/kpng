package userspace

import (
	"net"
	"strconv"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/kpng/api/localnetv1"
)

// BuildPortsToEndpointsMap builds a map of portname -> all ip:ports for that
// portname.
func buildPortsToEndpointsMap(ep *localnetv1.Endpoint, svc *localnetv1.Service) map[string][]string {
	portsToEndpoints := map[string][]string{}

	for _, ip := range ep.IPs.GetV4() {
		for _, port := range svc.Ports {
			if isValidEndpoint(ip, int(port.Port)) {
				portsToEndpoints[port.Name] = append(portsToEndpoints[port.Name], net.JoinHostPort(ip, strconv.Itoa(int(port.TargetPort))))

			}
		}
	}

	return portsToEndpoints
}

// isValidEndpoint checks that the given host / port pair are valid endpoint
func isValidEndpoint(host string, port int) bool {
	return host != "" && port > 0
}

// ShuffleStrings copies strings from the specified slice into a copy in random
// order. It returns a new slice.
func ShuffleStrings(s []string) []string {
	if s == nil {
		return nil
	}
	shuffled := make([]string, len(s))
	perm := utilrand.Perm(len(s))
	for i, j := range perm {
		shuffled[j] = s[i]
	}
	return shuffled
}
