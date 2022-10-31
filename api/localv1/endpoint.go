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

package localv1

import (
	"net"
)

// AddAddress adds an address to this endpoint, returning the parsed IP. `ìp` will be nil if it couldn't be parsed.
func (ep *Endpoint) AddAddress(s string) (ip net.IP) {
	if ep.IPs == nil {
		ep.IPs = &IPSet{}
	}

	return ep.IPs.Add(s)
}

func (ep *Endpoint) PortMapping(port *PortMapping) (target int32) {
	target = port.TargetPort
	if port.TargetPortName != "" {
		for _, override := range ep.PortOverrides {
			if override.Name == port.Name {
				target = override.Port
				break
			}
		}
	}
	return
}

func (ep *Endpoint) PortMappings(ports []*PortMapping) (mapping map[int32]int32) {
	mapping = make(map[int32]int32, len(ports))
	for _, port := range ports {
		mapping[port.Port] = ep.PortMapping(port)
	}
	return
}
func (ep *Endpoint) PortNameMappings(ports []*PortMapping) (mapping map[string]int32) {
	mapping = make(map[string]int32, len(ports))
	for _, port := range ports {
		mapping[port.Name] = ep.PortMapping(port)
	}
	return
}
