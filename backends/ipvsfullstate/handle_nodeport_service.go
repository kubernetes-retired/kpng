package ipvsfullsate

import (
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

		// iterate over NodeIPs
		for _, nodeIP := range getNodeIPs() {

			// STEP 2. create virtual server for NodeIP
			server := newVirtualServerForNodePort(nodeIP, service, portMapping)
			c.ipvsManager.ApplyServer(server)

			// STEP 3. add entry for NodeIP to [kubeNodePortSetTCP|kubeNodePortSetUDP|kubeNodePortSetSCTP]
			// depending on the protocol
			// TODO entry for kubeNodePortSetTCP & kubeNodePortSetUDP should be added once only as they will be same for every node, right now diffstore handles the duplicates.
			set = c.ipsetsManager.GetSetByName(getNodePortIpSetNameByProtocol(portMapping.Protocol))
			entry = newEntryForNodePort(nodeIP, portMapping)
			c.ipsetsManager.AddEntry(entry, set)

			// STEP 4. add entry for NodeIP to [kubeNodePortLocalSetTCP|kubeNodePortLocalSetUDP|kubeNodePortLocalSetSCTP]
			// depending on the protocol if external traffic policy is local
			// TODO entry for kubeNodePortLocalSetTCP & kubeNodePortLocalSetUDP should be added once only as they will be same for every node, right now diffstore handles the duplicates.
			if service.GetExternalTrafficToLocal() {
				set = c.ipsetsManager.GetSetByName(getNodePortLocalIpSetNameByProtocol(portMapping.Protocol))
				entry = newEntryForNodePort(nodeIP, portMapping)
				c.ipsetsManager.AddEntry(entry, set)
			}

			// iterate over service endpoints
			for _, endpoint := range endpoints {
				// iterate over EndpointIPs
				for _, endpointIp := range endpoint.IPs.V4 {

					// STEP 5. add endpoint as a destination to virtual server
					destination := newIpvsDestination(endpointIp, endpoint, portMapping)
					c.ipvsManager.AddDestination(destination, server)
				}
			}
		}
	}
}

// getNodePortIpSetNameByProtocol returns NodePort IPSet name for
// the given protocol, defaults to TCP.
func getNodePortIpSetNameByProtocol(protocol localv1.Protocol) string {
	switch protocol {
	case localv1.Protocol_SCTP:
		return kubeNodePortSetSCTP
	case localv1.Protocol_TCP:
		return kubeNodePortSetTCP
	case localv1.Protocol_UDP:
		return kubeNodePortSetUDP
	default:
		return kubeNodePortSetTCP
	}
}

// getNodePortLocalIpSetNameByProtocol returns NodePortLocal IPSet name for
// the given protocol, defaults to TCP.
func getNodePortLocalIpSetNameByProtocol(protocol localv1.Protocol) string {
	switch protocol {
	case localv1.Protocol_SCTP:
		return kubeNodePortLocalSetSCTP
	case localv1.Protocol_TCP:
		return kubeNodePortLocalSetTCP
	case localv1.Protocol_UDP:
		return kubeNodePortLocalSetUDP
	default:
		return kubeNodePortLocalSetTCP
	}
}
