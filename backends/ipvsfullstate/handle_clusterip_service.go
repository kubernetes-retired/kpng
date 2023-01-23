package ipvsfullsate

import (
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/ipsets"
	"sigs.k8s.io/kpng/client"
)

// addServiceEndpointsForClusterIP breaks down *client.ServiceEndpoints into multiple network
// objects which needs to be programmed in the kernel to achieve the desired data path.
func (c *Controller) addServiceEndpointsForClusterIP(serviceEndpoints *client.ServiceEndpoints) {
	var entry *ipsets.Entry
	var set *ipsets.Set

	service := serviceEndpoints.Service
	endpoints := serviceEndpoints.Endpoints

	// iterate over service ports
	for _, portMapping := range service.Ports {

		// iterate over ClusterIPs
		for _, clusterIP := range getClusterIPs(service) {

			// STEP 1. create virtual server for Cluster IP
			server := newVirtualServerForClusterIP(clusterIP, service, portMapping)
			c.ipvsManager.ApplyServer(server)

			// STEP 2. add entry for ClusterIP To kubeClusterIPSet
			entry = newEntryForClusterIP(clusterIP, portMapping)
			set = c.ipsetsManager.GetSetByName(kubeClusterIPSet)
			c.ipsetsManager.AddEntry(entry, set)

			// STEP 3. bind the ClusterIP to Host Interface
			c.ipvsManager.BindServerToInterface(server)

			// iterate over service endpoint
			for _, endpoint := range endpoints {
				// iterate over EndpointIPs
				for _, endpointIp := range endpoint.IPs.V4 {

					// STEP 4. add endpoint as a destination to virtual server
					destination := newIpvsDestination(endpointIp, endpoint, portMapping)
					c.ipvsManager.AddDestination(destination, server)

					if endpoint.GetLocal() {
						// STEP 5. Add entry for EndpointIP to kubeLoopBackIPSet if endpoint is local
						entry = newEntryForLocalEndpoint(endpointIp, endpoint, portMapping)
						set = c.ipsetsManager.GetSetByName(kubeLoopBackIPSet)
						c.ipsetsManager.AddEntry(entry, set)
					}
				}
			}
		}

		// iterate over ExternalIPs
		for _, externalIP := range getExternalIPs(service) {

			// STEP 6. create virtual server for ExternalIP
			server := newVirtualServerForExternalIP(externalIP, service, portMapping)
			c.ipvsManager.ApplyServer(server)

			// create entry for ExternalIP
			entry = newEntryForExternalIP(externalIP, portMapping)

			// STEP 7. add entry for ExternalIP to kubeExternalIPSet
			set = c.ipsetsManager.GetSetByName(kubeExternalIPSet)
			c.ipsetsManager.AddEntry(entry, set)

			// STEP 8. add entry for ExternalIP to kubeExternalIPLocalSet if external traffic policy is local
			if service.GetExternalTrafficToLocal() {
				set = c.ipsetsManager.GetSetByName(kubeExternalIPLocalSet)
				c.ipsetsManager.AddEntry(entry, set)
			}

			// STEP 9. bind the ExternalIP to Host Interface
			c.ipvsManager.BindServerToInterface(server)

			// iterate over service endpoints
			for _, endpoint := range endpoints {
				// iterate over EndpointIPs
				for _, endpointIp := range endpoint.IPs.V4 {

					// STEP 10. add endpoint as a destination to virtual server
					destination := newIpvsDestination(endpointIp, endpoint, portMapping)
					c.ipvsManager.AddDestination(destination, server)

					if endpoint.GetLocal() {
						// STEP 11. Add entry for EndpointIP to kubeLoopBackIPSet if endpoint is local
						entry = newEntryForLocalEndpoint(endpointIp, endpoint, portMapping)
						set = c.ipsetsManager.GetSetByName(kubeLoopBackIPSet)
						c.ipsetsManager.AddEntry(entry, set)
					}
				}
			}
		}
	}
}
