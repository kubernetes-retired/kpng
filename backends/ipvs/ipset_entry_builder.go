package ipvs

import (
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/backends/ipvs/internal/ipsets"
)

// newEntryForClusterIP creates entry for ClusterIP for kubeClusterIPSet.
func newEntryForClusterIP(clusterIP string, portMapping *localv1.PortMapping) *ipsets.Entry {
	return &ipsets.Entry{
		IP:       clusterIP,
		Port:     int(portMapping.GetPort()),
		Protocol: portMapping.Protocol,
		SetType:  ipsets.HashIPPort,
	}
}

// newEntryForExternalIP creates entry for ExternalIP for kubeExternalIPSet.
func newEntryForExternalIP(externalIP string, portMapping *localv1.PortMapping) *ipsets.Entry {
	return &ipsets.Entry{
		IP:       externalIP,
		Port:     int(portMapping.GetPort()),
		Protocol: portMapping.Protocol,
		SetType:  ipsets.HashIPPort,
	}
}

// newEntryForLoadBalancerSourceRange creates entry for ExternalIP for kubeExternalIPSet.
func newEntryForLoadBalancerSourceRange(loadBalancerIP string, sourceRange string, portMapping *localv1.PortMapping) *ipsets.Entry {
	return &ipsets.Entry{
		IP:       loadBalancerIP,
		Port:     int(portMapping.GetPort()),
		Protocol: portMapping.Protocol,
		SetType:  ipsets.HashIPPortNet,
		Net:      sourceRange,
	}
}

// newEntryForLoadBalancer creates entry for LoadBalancer for kubeLoadBalancerSet.
func newEntryForLoadBalancer(loadBalancerIP string, portMapping *localv1.PortMapping) *ipsets.Entry {
	return &ipsets.Entry{
		IP:       loadBalancerIP,
		Port:     int(portMapping.GetPort()),
		Protocol: portMapping.Protocol,
		SetType:  ipsets.HashIPPort,
	}
}

// newEntryForLocalEndpoint creates entry for local EndpointIP for kubeLoopBackIPSet.
func newEntryForLocalEndpoint(endpointIP string, endpoint *localv1.Endpoint, portMapping *localv1.PortMapping) *ipsets.Entry {
	return &ipsets.Entry{
		IP:       endpointIP,
		Port:     int(getTargetPort(endpoint, portMapping)),
		Protocol: portMapping.Protocol,
		IP2:      endpointIP,
		SetType:  ipsets.HashIPPortIP,
	}

}

// newEntryForNodePortNonSCTP creates entry for NodePort for kubeNodePortSetTCP | kubeNodePortSetUDP.
func newEntryForNodePortNonSCTP(portMapping *localv1.PortMapping) *ipsets.Entry {

	return &ipsets.Entry{
		Port:     int(portMapping.GetNodePort()),
		Protocol: portMapping.Protocol,
		SetType:  ipsets.BitmapPort,
	}
}

// newEntryForNodePortNonSCTP creates entry for NodePort for kubeNodePortSetSCTP
func newEntryForNodePortSCTP(nodeIP string, portMapping *localv1.PortMapping) *ipsets.Entry {
	return &ipsets.Entry{
		IP:       nodeIP,
		Port:     int(portMapping.GetNodePort()),
		Protocol: portMapping.Protocol,
		SetType:  ipsets.HashIPPort,
	}
}

// newEntryForNodePort creates entry for NodePort for kubeNodePortSets
func newEntryForNodePort(nodeIP string, portMapping *localv1.PortMapping) *ipsets.Entry {
	switch portMapping.Protocol {
	case localv1.Protocol_SCTP:
		return newEntryForNodePortSCTP(nodeIP, portMapping)
	default:
		return newEntryForNodePortNonSCTP(portMapping)
	}
}
