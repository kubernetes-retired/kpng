/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package userspace

import (
	"net"
	"strconv"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/kpng/api/localv1"
)

// BuildPortsToEndpointsMap builds a map of portname -> all ip:ports for that
// portname.
func buildPortsToEndpointsMap(ep *localv1.Endpoint, svc *localv1.Service) map[string][]string {
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
