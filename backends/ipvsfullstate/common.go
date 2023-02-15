package ipvsfullsate

import (
	v1 "k8s.io/api/core/v1"
	netutils "k8s.io/utils/net"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/ipvs"
)

// getClusterIPs returns ClusterIPs for given IPFamily associated with the service.
func getClusterIPs(service *localv1.Service, ipFamily v1.IPFamily) []string {
	IPs := make([]string, 0)
	if service.IPs.ClusterIPs != nil {
		if ipFamily == v1.IPv4Protocol {
			return service.IPs.ClusterIPs.GetV4()
		} else if ipFamily == v1.IPv6Protocol {
			return service.IPs.ClusterIPs.GetV6()
		}
	}
	return IPs
}

// getExternalIPs safely returns ExternalIPs for given IPFamily associated with the service.
func getExternalIPs(service *localv1.Service, ipFamily v1.IPFamily) []string {
	IPs := make([]string, 0)
	if service.IPs.ExternalIPs != nil {
		if ipFamily == v1.IPv4Protocol {
			return service.IPs.ExternalIPs.GetV4()
		} else if ipFamily == v1.IPv6Protocol {
			return service.IPs.ExternalIPs.GetV6()
		}
	}
	return IPs
}

// getNodeIPs safely returns all Node IPs for given IPFamily.
func getNodeIPs(ipFamily v1.IPFamily) []string {
	IPs := make([]string, 0)
	for _, ip := range *NodeAddresses {
		if ipFamily == v1.IPv4Protocol && netutils.IsIPv4String(ip) {
			IPs = append(IPs, ip)
		} else if ipFamily == v1.IPv6Protocol && netutils.IsIPv6String(ip) {
			IPs = append(IPs, ip)
		}
	}
	return IPs
}

// getLoadBalancerIPs safely returns LoadBalancerIPs for given IPFamily associated with the service.
func getLoadBalancerIPs(service *localv1.Service, ipFamily v1.IPFamily) []string {
	IPs := make([]string, 0)
	if service.IPs.LoadBalancerIPs != nil {
		if ipFamily == v1.IPv4Protocol {
			return service.IPs.LoadBalancerIPs.GetV4()
		} else if ipFamily == v1.IPv6Protocol {
			return service.IPs.LoadBalancerIPs.GetV6()
		}
	}
	return IPs
}

// getEndpointIPs returns EndpointIPs for given IPFamily associated with the endpoint.
func getEndpointIPs(endpoint *localv1.Endpoint, ipFamily v1.IPFamily) []string {
	IPs := make([]string, 0)
	if endpoint.IPs != nil {
		if ipFamily == v1.IPv4Protocol {
			return endpoint.IPs.GetV4()
		} else if ipFamily == v1.IPv6Protocol {
			return endpoint.IPs.GetV6()
		}
	}
	return IPs
}

// getSourceRangesForLoadBalancer safely returns sourceRanges associated with the service.
func getSourceRangesForLoadBalancer(service *localv1.Service) []string {
	sourceRanges := make([]string, 0)
	for _, ip := range service.IPFilters {
		if len(ip.SourceRanges) > 0 {
			for _, srcIP := range ip.SourceRanges {
				sourceRanges = append(sourceRanges, srcIP)
			}
		}
	}
	return sourceRanges
}

// getSessionAffinity returns the session affinity associated with the service. Right now we
// only support affinity on ClientIP
func getSessionAffinity(affinity interface{}) SessionAffinity {
	var sessionAffinity SessionAffinity
	switch affinity.(type) {
	case *localv1.Service_ClientIP:
		sessionAffinity.ClientIP = affinity.(*localv1.Service_ClientIP)
	}
	return sessionAffinity
}

// getTimeoutForClientIPAffinity returns timeout associated with service for virtual
// server if the service has ClientIP session affinity.
func getTimeoutForClientIPAffinity(service *localv1.Service) int32 {
	affinity := getSessionAffinity(service.SessionAffinity)
	if affinity.ClientIP != nil {
		return affinity.ClientIP.ClientIP.TimeoutSeconds
	}
	return 0
}

// newVirtualServer returns virtual server for the given arguments.
func newVirtualServer(ip string, service *localv1.Service, portMapping *localv1.PortMapping) *ipvs.VirtualServer {
	timeout := getTimeoutForClientIPAffinity(service)
	return &ipvs.VirtualServer{
		IP:       ip,
		Port:     portMapping.Port,
		Protocol: portMapping.Protocol,
		Timeout:  timeout,
	}
}

// newVirtualServerForClusterIP returns virtual server for ClusterIP service.
func newVirtualServerForClusterIP(ip string, service *localv1.Service, portMapping *localv1.PortMapping) *ipvs.VirtualServer {
	return newVirtualServer(ip, service, portMapping)
}

// newVirtualServerForExternalIP returns virtual server for ExternalIP service.
func newVirtualServerForExternalIP(ip string, service *localv1.Service, portMapping *localv1.PortMapping) *ipvs.VirtualServer {
	return newVirtualServer(ip, service, portMapping)
}

// newVirtualServerForNodePort returns virtual server for NodePort service.
func newVirtualServerForNodePort(ip string, service *localv1.Service, portMapping *localv1.PortMapping) *ipvs.VirtualServer {
	timeout := getTimeoutForClientIPAffinity(service)
	return &ipvs.VirtualServer{
		IP:       ip,
		Port:     portMapping.NodePort,
		Protocol: portMapping.Protocol,
		Timeout:  timeout,
	}
}

// newVirtualServerForLoadBalancer returns virtual server for LoadBalancer service.
func newVirtualServerForLoadBalancer(ip string, service *localv1.Service, portMapping *localv1.PortMapping) *ipvs.VirtualServer {
	return newVirtualServer(ip, service, portMapping)
}

// getTargetPort returns port for endpoint, supports both single and multi port services.
func getTargetPort(endpoint *localv1.Endpoint, portMapping *localv1.PortMapping) int32 {
	portNameToValue := make(map[string]int32)

	for _, portOverride := range endpoint.PortOverrides {
		portNameToValue[portOverride.Name] = portOverride.Port
	}

	targetPort := portMapping.TargetPort
	if targetPort == 0 {
		targetPort = portNameToValue[portMapping.Name]
	}
	return targetPort
}

// newIpvsDestination return endpoints as destination (real server) for the virtual server.
func newIpvsDestination(ip string, endpoint *localv1.Endpoint, portMapping *localv1.PortMapping) *ipvs.Destination {
	targetPort := getTargetPort(endpoint, portMapping)
	return &ipvs.Destination{
		IP:   ip,
		Port: targetPort,
	}
}
