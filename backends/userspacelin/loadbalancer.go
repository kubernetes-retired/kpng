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
