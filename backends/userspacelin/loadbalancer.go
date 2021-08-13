package userspacelin

import (
	v1 "k8s.io/api/core/v1"
	"net"
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

	iptables.EndpointsHandler
}
