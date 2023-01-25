package ipvsfullsate

import (
	ipsets2 "sigs.k8s.io/kpng/backends/ipvsfullstate/internal/ipsets"
	"sigs.k8s.io/kpng/client"
)

// addServiceEndpointsForLoadBalancer breaks down *client.ServiceEndpoints into multiple network
// objects which needs to be programmed in the kernel to achieve the desired data path.
func (c *Controller) addServiceEndpointsForLoadBalancer(serviceEndpoints *client.ServiceEndpoints) {

	// STEP 1. create all NodePort components required by LoadBalancer service, which
	// will further create ClusterIP components.
	c.addServiceEndpointsForNodePort(serviceEndpoints)

	var entry *ipsets2.Entry
	var set *ipsets2.Set

	service := serviceEndpoints.Service
	endpoints := serviceEndpoints.Endpoints

	// iterate over service ports
	for _, portMapping := range service.Ports {

		// iterate over LoadBalancerIPs
		for _, loadBalancerIP := range getLoadBalancerIPs(service) {

			// STEP 2. create virtual server for LoadBalancerIP
			server := newVirtualServerForLoadBalancer(loadBalancerIP, service, portMapping)
			c.ipvsManager.ApplyServer(server)

			// STEP 3. add entry for LoadBalancerIP to kubeLoadBalancerSet
			set = c.ipsetsManager.GetSetByName(kubeLoadBalancerSet)
			entry = newEntryForLoadBalancer(loadBalancerIP, portMapping)
			c.ipsetsManager.AddEntry(entry, set)

			// STEP 4. add entry for LoadBalancerIP to kubeLoadBalancerLocalSet,
			// if external traffic policy is local
			if service.GetExternalTrafficToLocal() {
				set = c.ipsetsManager.GetSetByName(kubeLoadBalancerLocalSet)
				entry = newEntryForLoadBalancer(loadBalancerIP, portMapping)
				c.ipsetsManager.AddEntry(entry, set)
			}

			// STEP 5. add entry for LoadBalancerIP and SourceRanges to kubeLoadBalancerSourceCIDRSet
			for _, sourceRange := range getSourceRangesForLoadBalancer(service) {
				set = c.ipsetsManager.GetSetByName(kubeLoadBalancerSourceCIDRSet)
				entry = newEntryForLoadBalancerSourceRange(loadBalancerIP, sourceRange, portMapping)
				c.ipsetsManager.AddEntry(entry, set)
			}

			// STEP 6. add entry for LoadBalancerIP to kubeLoadbalancerFWSet, if source ranges is configured
			if len(getSourceRangesForLoadBalancer(service)) > 0 {
				set = c.ipsetsManager.GetSetByName(kubeLoadbalancerFWSet)
				entry = newEntryForLoadBalancer(loadBalancerIP, portMapping)
				c.ipsetsManager.AddEntry(entry, set)
			}

			// TODO entries to kubeLoadBalancerSourceIPSet; take reference from upstream ipvs proxier

			// iterate over service endpoints
			for _, endpoint := range endpoints {
				// iterate over EndpointIPs
				for _, endpointIp := range endpoint.IPs.V4 {
					// STEP 7. add endpoint as a destination to virtual server
					destination := newIpvsDestination(endpointIp, endpoint, portMapping)
					c.ipvsManager.AddDestination(destination, server)

				}
			}
		}
	}
}
