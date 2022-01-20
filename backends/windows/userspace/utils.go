package main

import (
	"net"
	"sigs.k8s.io/kpng/api/localnetv1"
	"strconv"
)

// BuildPortsToEndpointsMap builds a map of portname -> all ip:ports for that
// portname.
func buildPortsToEndpointsMap(ep *localnetv1.Endpoint, svc *localnetv1.Service) map[string][]string {
	portsToEndpoints := map[string][]string{}

	for _, ip := range ep.IPs.GetV4() {
		for _, port := range svc.Ports {
			if isValidEndpoint(ip, int(port.Port)) {
				portsToEndpoints[port.Name] = append(portsToEndpoints[port.Name], net.JoinHostPort(ip, strconv.Itoa(int(port.Port))))

			}
		}
	}

	return portsToEndpoints
}

// isValidEndpoint checks that the given host / port pair are valid endpoint
func isValidEndpoint(host string, port int) bool {
	return host != "" && port > 0
}
