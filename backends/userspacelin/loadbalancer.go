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

package userspacelin

import (
	"net"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/backends/iptables"
)

// LoadBalancer is an interface for distributing incoming requests to service endpoints.
type LoadBalancer interface {
	// NextEndpoint returns the endpoint to handle a request for the given
	// service-port and source address.
	NextEndpoint(service iptables.ServicePortName, srcAddr net.Addr, sessionAffinityReset bool) (string, error)
	NewService(service iptables.ServicePortName, affinityClientIP *localnetv1.ClientIPAffinity, stickyMaxAgeSeconds int) error
	DeleteService(service iptables.ServicePortName)
	CleanupStaleStickySessions(service iptables.ServicePortName)
	ServiceHasEndpoints(service iptables.ServicePortName) bool

	// For userspace because we dont have an EndpointChangeTracker which can auto lookup services behind the scenes,
	// we need to send this explicitly.
	OnEndpointsAdd(ep *localnetv1.Endpoint, svc *localnetv1.Service)
	OnEndpointsDelete(ep *localnetv1.Endpoint, svc *localnetv1.Service)
	OnEndpointsSynced()
}
