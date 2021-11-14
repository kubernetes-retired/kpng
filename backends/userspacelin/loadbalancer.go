package userspacelin

import (
	"net"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/backends/iptables"
)

// LoadBalancer is an interface for distributing incoming requests to service endpoints.
type LoadBalancer interface {
	// NextEndpoint returns the endpoint to handle a request for the given
	// service-port and source address.
	NextEndpoint(service iptables.ServicePortName, srcAddr net.Addr, sessionAffinityReset bool) (string, error)
	NewService(service iptables.ServicePortName, sessionAffinityType v1.ServiceAffinity, stickyMaxAgeSeconds int) error
	DeleteService(service iptables.ServicePortName)
	CleanupStaleStickySessions(service iptables.ServicePortName)
	ServiceHasEndpoints(service iptables.ServicePortName) bool

	// For userspace because we dont have an EndpointChangeTracker which can auto lookup services behind the scenes,
	// we need to send this explicitly.
	OnEndpointsAdd(service []*iptables.ServicePortName, endpoints *localnetv1.Endpoint)
	OnEndpointsUpdate(oldEndpoints, endpoints *v1.Endpoints)
	OnEndpointsDelete(endpoints *v1.Endpoints)
	OnEndpointsSynced()
}
