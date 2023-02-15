package ipvsfullsate

import (
	v1 "k8s.io/api/core/v1"
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

		// iterate over ipFamily
		for _, ipFamily := range []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol} {

			// iterate over LoadBalancerIPs
			for _, loadBalancerIP := range getLoadBalancerIPs(service, ipFamily) {

				// STEP 2. create virtual server for LoadBalancerIP
				server := newVirtualServerForLoadBalancer(loadBalancerIP, service, portMapping)
				c.ipvsManager.ApplyServer(server)

				// STEP 3. add entry for LoadBalancerIP to kubeLoadBalancerIPSet
				set = c.ipsetsManager.GetSetByName(kubeLoadBalancerIPSet[ipFamily])
				entry = newEntryForLoadBalancer(loadBalancerIP, portMapping)
				c.ipsetsManager.AddEntry(entry, set)

				// STEP 4. add entry for LoadBalancerIP to kubeLoadBalancerLocalIPSet,
				// if external traffic policy is local
				if service.GetExternalTrafficToLocal() {
					set = c.ipsetsManager.GetSetByName(kubeLoadBalancerLocalIPSet[ipFamily])
					entry = newEntryForLoadBalancer(loadBalancerIP, portMapping)
					c.ipsetsManager.AddEntry(entry, set)
				}

				// STEP 5. add entry for LoadBalancerIP and SourceRanges to kubeLoadBalancerSourceCIDRIPSet
				for _, sourceRange := range getSourceRangesForLoadBalancer(service) {
					set = c.ipsetsManager.GetSetByName(kubeLoadBalancerSourceCIDRIPSet[ipFamily])
					entry = newEntryForLoadBalancerSourceRange(loadBalancerIP, sourceRange, portMapping)
					c.ipsetsManager.AddEntry(entry, set)
				}

				// STEP 6. add entry for LoadBalancerIP to kubeLoadbalancerFWIPSet, if source ranges is configured
				if len(getSourceRangesForLoadBalancer(service)) > 0 {
					set = c.ipsetsManager.GetSetByName(kubeLoadbalancerFWIPSet[ipFamily])
					entry = newEntryForLoadBalancer(loadBalancerIP, portMapping)
					c.ipsetsManager.AddEntry(entry, set)
				}

				// TODO entries to kubeLoadBalancerSourceIPSet; take reference from upstream ipvs proxier

				// iterate over service endpoints
				for _, endpoint := range endpoints {
					// iterate over EndpointIPs
					for _, endpointIp := range getEndpointIPs(endpoint, ipFamily) {
						// STEP 7. add endpoint as a destination to virtual server
						destination := newIpvsDestination(endpointIp, endpoint, portMapping)
						c.ipvsManager.AddDestination(destination, server)

					}
				}
			}
		}
	}
}
