/*
Copyright 2016 The Kubernetes Authors.

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

	"sigs.k8s.io/kpng/api/localv1"
)

// EndpointsHandler is an abstract interface of objects which receive
// notifications about endpoints object changes.
type EndpointsHandler interface {
	// OnEndpointsAdd is called whenever creation of new endpoints object
	// is observed.
	OnEndpointsAdd(ep *localv1.Endpoint, svc *localv1.Service)
	// OnEndpointsDelete is called whenever deletion of an existing endpoints
	// object is observed.
	OnEndpointsDelete(ep *localv1.Endpoint, svc *localv1.Service)
	// OnEndpointsSynced is called once all the initial event handlers were
	// called and the state is fully propagated to local cache.
	OnEndpointsSynced()
}

// LoadBalancer is an interface for distributing incoming requests to service endpoints.
type LoadBalancer interface {
	// NextEndpoint returns the endpoint to handle a request for the given
	// service-port and source address.
	NextEndpoint(service ServicePortName, srcAddr net.Addr, sessionAffinityReset bool) (string, error)
	NewService(service ServicePortName, affinityClientIP *localv1.ClientIPAffinity, stickyMaxAgeMinutes int) error
	DeleteService(service ServicePortName)
	CleanupStaleStickySessions(service ServicePortName)

	EndpointsHandler
}
