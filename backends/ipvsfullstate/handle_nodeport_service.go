package ipvsfullsate

import (
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/ipsets"
	"sigs.k8s.io/kpng/client"
)

// addServiceEndpointsForNodePort breaks down *client.ServiceEndpoints into multiple network
// objects which needs to be programmed in the kernel to achieve the desired data path.
func (c *Controller) addServiceEndpointsForNodePort(serviceEndpoints *client.ServiceEndpoints) {

	// STEP 1. create all ClusterIP components required by NodePort service
	c.addServiceEndpointsForClusterIP(serviceEndpoints)

	var entry *ipsets.Entry
	var set *ipsets.Set

	service := serviceEndpoints.Service
	endpoints := serviceEndpoints.Endpoints

	// iterate over service ports
	for _, portMapping := range service.Ports {

		// iterate over ipFamily
		for _, ipFamily := range []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol} {

			// iterate over NodeIPs
			for _, nodeIP := range getNodeIPs(ipFamily) {

				// STEP 2. create virtual server for NodeIP
				server := newVirtualServerForNodePort(nodeIP, service, portMapping)
				c.ipvsManager.ApplyServer(server)

				// STEP 3. add entry for NodeIP to [kubeNodePortTCPIPSet|kubeNodePortUDPIPSet|kubeNodePortSCTPIPSet]
				// depending on the protocol
				// TODO entry for kubeNodePortTCPIPSet & kubeNodePortUDPIPSet should be added once only as they will be same for every node, right now diffstore handles the duplicates.
				set = c.ipsetsManager.GetSetByName(getNodePortIpSetNameByProtocol(ipFamily, portMapping.Protocol))
				entry = newEntryForNodePort(nodeIP, portMapping)
				c.ipsetsManager.AddEntry(entry, set)

				// STEP 4. add entry for NodeIP to [kubeNodePortLocalTCPIPSet|kubeNodePortLocalUDPIPSet|kubeNodePortLocalSCTPIPSet]
				// depending on the protocol if external traffic policy is local
				// TODO entry for kubeNodePortLocalTCPIPSet & kubeNodePortLocalUDPIPSet should be added once only as they will be same for every node, right now diffstore handles the duplicates.
				if service.GetExternalTrafficToLocal() {
					entry = newEntryForNodePort(nodeIP, portMapping)
					set = c.ipsetsManager.GetSetByName(getNodePortLocalIpSetNameByProtocol(ipFamily, portMapping.Protocol))
					c.ipsetsManager.AddEntry(entry, set)
				}

				// iterate over service endpoints
				for _, endpoint := range endpoints {
					// iterate over EndpointIPs
					for _, endpointIp := range getEndpointIPs(endpoint, ipFamily) {

						// STEP 5. add endpoint as a destination to virtual server
						destination := newIpvsDestination(endpointIp, endpoint, portMapping)
						c.ipvsManager.AddDestination(destination, server)
					}
				}
			}
		}
	}
}

// getNodePortIpSetNameByProtocol returns NodePort IPSet name for
// the given ipFamily and protocol, defaults to TCP.
func getNodePortIpSetNameByProtocol(ipFamily v1.IPFamily, protocol localv1.Protocol) string {
	switch protocol {
	case localv1.Protocol_SCTP:
		return kubeNodePortSCTPIPSet[ipFamily]
	case localv1.Protocol_TCP:
		return kubeNodePortTCPIPSet[ipFamily]
	case localv1.Protocol_UDP:
		return kubeNodePortUDPIPSet[ipFamily]
	default:
		return kubeNodePortTCPIPSet[ipFamily]
	}
}

// getNodePortLocalIpSetNameByProtocol returns NodePortLocal IPSet name for
// the given ipFamily and protocol, defaults to TCP.
func getNodePortLocalIpSetNameByProtocol(ipFamily v1.IPFamily, protocol localv1.Protocol) string {
	switch protocol {
	case localv1.Protocol_SCTP:
		return kubeNodePortLocalSCTPIPSet[ipFamily]
	case localv1.Protocol_TCP:
		return kubeNodePortLocalTCPIPSet[ipFamily]
	case localv1.Protocol_UDP:
		return kubeNodePortLocalUDPIPSet[ipFamily]
	default:
		return kubeNodePortLocalTCPIPSet[ipFamily]
	}
}
