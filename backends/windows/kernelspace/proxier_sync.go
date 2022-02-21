//go:build windows
// +build windows

/*
Copyright 2017-2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kernelspace

import (
	"strings"
	"sync/atomic"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	//	"k8s.io/kubernetes/pkg/proxy/metrics"
	netutils "k8s.io/utils/net"
)

// Sync is called to synchronize the Proxier state to hns as soon as possible.
func (Proxier *Proxier) Sync() {
	//	if Proxier.healthzServer != nil {
	//		Proxier.healthzServer.QueuedUpdate()
	//	}

	// TODO commenting out metrics, Jay to fix , figure out how to  copy these later, avoiding pkg/proxy imports
	// metrics.SyncProxyRulesLastQueuedTimestamp.SetToCurrentTime()
	Proxier.syncRunner.Run()
}

// SyncLoop runs periodic work.  This is expected to run as a goroutine or as the main loop of the app.  It does not return.
func (Proxier *Proxier) SyncLoop() {
	// Update healthz timestamp at beginning in case Sync() never succeeds.
	//	if Proxier.healthzServer != nil {
	//		Proxier.healthzServer.Updated()
	//	}
	// synthesize "last change queued" time as the informers are syncing.
	//	metrics.SyncProxyRulesLastQueuedTimestamp.SetToCurrentTime()
	Proxier.syncRunner.Loop(wait.NeverStop)
}

func (Proxier *Proxier) isInitialized() bool {
	return atomic.LoadInt32(&Proxier.initialized) > 0
}

func (Proxier *Proxier) cleanupAllPolicies() {
	for svcName, svc := range Proxier.serviceMap {
		svcInfo, ok := svc.(*serviceInfo)
		if !ok {
			klog.ErrorS(nil, "Failed to cast serviceInfo", "serviceName", svcName)
			continue
		}
		endpoints := Proxier.endpointsMap[svcName.NamespacedName]
		for _, e := range *endpoints {
			svcInfo.cleanupAllPolicies(e)
		}
	}
}

// This is where all of the hns save/restore calls happen.
// assumes Proxier.mu is held
func (Proxier *Proxier) syncProxyRules() {
	Proxier.mu.Lock()
	defer Proxier.mu.Unlock()

	// don't sync rules till we've received services and windowsEndpoint
	if !Proxier.isInitialized() {
		klog.V(2).InfoS("Not syncing hns until Services and Endpoints have been received from master")
		return
	}

	// Keep track of how long syncs take.
	start := time.Now()
	defer func() {

		//		metrics.SyncProxyRulesLatency.Observe(metrics.SinceInSeconds(start))
		klog.V(4).InfoS("Syncing proxy rules complete", "elapsed", time.Since(start))
	}()

	hnsNetworkName := Proxier.network.name
	hns := Proxier.hns

	prevNetworkID := Proxier.network.id
	updatedNetwork, err := hns.getNetworkByName(hnsNetworkName)
	if updatedNetwork == nil || updatedNetwork.id != prevNetworkID || isNetworkNotFoundError(err) {
		klog.InfoS("The HNS network is not present or has changed since the last sync, please check the CNI deployment", "hnsNetworkName", hnsNetworkName)
		Proxier.cleanupAllPolicies()
		if updatedNetwork != nil {
			Proxier.network = *updatedNetwork
		}
		return
	}

	// We assume that if this was called, we really want to sync them,
	// even if nothing changed in the meantime. In other words, callers are
	// responsible for detecting no-op changes and not calling this function.
	serviceUpdateResult := Proxier.serviceMap.Update(Proxier.serviceChanges)
	endpointUpdateResult := Proxier.endpointsMap.Update(Proxier.endpointsChanges)

	staleServices := serviceUpdateResult.UDPStaleClusterIP
	// merge stale services gathered from updateEndpointsMap
	for _, svcPortName := range endpointUpdateResult.StaleServiceNames {
		if svcInfo, ok := Proxier.serviceMap[svcPortName]; ok && svcInfo != nil && svcInfo.Protocol() == v1.ProtocolUDP {
			klog.V(2).InfoS("Stale udp service", "servicePortName", svcPortName, "clusterIP", svcInfo.ClusterIP())
			staleServices.Insert(svcInfo.ClusterIP().String())
		}
	}

	if strings.EqualFold(Proxier.network.networkType, NETWORK_TYPE_OVERLAY) {
		existingSourceVip, err := hns.getEndpointByIpAddress(Proxier.sourceVip, hnsNetworkName)
		if existingSourceVip == nil {
			_, err = newSourceVIP(hns, hnsNetworkName, Proxier.sourceVip, Proxier.hostMac, Proxier.nodeIP.String())
		}
		if err != nil {
			klog.ErrorS(err, "Source Vip endpoint creation failed")
			return
		}
	}

	klog.V(3).InfoS("Syncing Policies")

	// Program HNS by adding corresponding policies for each service.
	for svcName, svc := range Proxier.serviceMap {
		svcInfo, ok := svc.(*serviceInfo)
		if !ok {
			klog.ErrorS(nil, "Failed to cast serviceInfo", "serviceName", svcName)
			continue
		}

		if svcInfo.policyApplied {
			klog.V(4).InfoS("Policy already applied", "serviceInfo", svcInfo)
			continue
		}

		if strings.EqualFold(Proxier.network.networkType, NETWORK_TYPE_OVERLAY) {
			serviceVipEndpoint, _ := hns.getEndpointByIpAddress(svcInfo.ClusterIP().String(), hnsNetworkName)
			if serviceVipEndpoint == nil {
				klog.V(4).InfoS("No existing remote endpoint", "IP", svcInfo.ClusterIP())
				hnsEndpoint := &windowsEndpoint{
					ip:              svcInfo.ClusterIP().String(),
					isLocal:         false,
					macAddress:      Proxier.hostMac,
					providerAddress: Proxier.nodeIP.String(),
				}

				newHnsEndpoint, err := hns.createEndpoint(hnsEndpoint, hnsNetworkName)
				if err != nil {
					klog.ErrorS(err, "Remote endpoint creation failed for service VIP")
					continue
				}

				newHnsEndpoint.refCount = Proxier.endPointsRefCount.getRefCount(newHnsEndpoint.hnsID)
				*newHnsEndpoint.refCount++
				svcInfo.remoteEndpoint = newHnsEndpoint
			}
		}

		var hnsEndpoints []windowsEndpoint
		var hnsLocalEndpoints []windowsEndpoint
		klog.V(4).InfoS("Applying Policy", "serviceInfo", svcName)
		// Create Remote windowsEndpoint for every endpoint, corresponding to the service
		containsPublicIP := false
		containsNodeIP := false

		for _, epInfo := range *Proxier.endpointsMap[svcName.NamespacedName] {

			// !!!!!!!!!!!!!!!!
			// ep must be an instance of "windowsEndpoint..."
			// it is an localvnet1.Endpoint...
			//			ep, ok := epInfo.(*windowsEndpoint)
			// !!!!!!!!!!!!!!!!
			ep := epInfo
			if !ok {
				klog.ErrorS(nil, "Failed to cast windowsEndpoint", "serviceName", svcName)
				continue
			}

			// TODO NEed to see how to implement this in localvent1... do we need add it to the brain ?  -> jay
			//if !ep.IsReady() {
			//	continue
			//}

			var newHnsEndpoint *windowsEndpoint
			hnsNetworkName := Proxier.network.name
			var err error

			// targetPort is zero if it is specified as a name in port.TargetPort, so the real port should be got from windowsEndpoint.
			// Note that hcsshim.AddLoadBalancer() doesn't support windowsEndpoint with different ports, so only port from first endpoint is used.
			// TODO(feiskyer): add support of different endpoint ports after hcsshim.AddLoadBalancer() add that.
			// TODO jay : disabling this logic
			if svcInfo.targetPort == 0 {
				svcInfo.targetPort = int(ep.port)
			}

			if len(ep.hnsID) > 0 {
				newHnsEndpoint, err = hns.getEndpointByID(ep.hnsID)
			}

			if newHnsEndpoint == nil {
				// First check if an endpoint resource exists for this IP, on the current host
				// A Local endpoint could exist here already
				// A remote endpoint was already created and proxy was restarted
				newHnsEndpoint, err = hns.getEndpointByIpAddress(ep.IP(), hnsNetworkName)
			}
			if newHnsEndpoint == nil {
				if ep.GetIsLocal() {
					klog.ErrorS(err, "Local endpoint not found: on network", "ip", ep.IP(), "hnsNetworkName", hnsNetworkName)
					continue
				}

				if strings.EqualFold(Proxier.network.networkType, NETWORK_TYPE_OVERLAY) {
					klog.InfoS("Updating network to check for new remote subnet policies", "networkName", Proxier.network.name)
					networkName := Proxier.network.name
					updatedNetwork, err := hns.getNetworkByName(networkName)
					if err != nil {
						klog.ErrorS(err, "Unable to find HNS Network specified, please check network name and CNI deployment", "hnsNetworkName", hnsNetworkName)
						Proxier.cleanupAllPolicies()
						return
					}
					Proxier.network = *updatedNetwork
					providerAddress := Proxier.network.findRemoteSubnetProviderAddress(ep.IP())
					if len(providerAddress) == 0 {
						klog.InfoS("Could not find provider address, assuming it is a public IP", "IP", ep.IP())
						providerAddress = Proxier.nodeIP.String()
					}

					hnsEndpoint := &windowsEndpoint{
						ip:              ep.ip,
						isLocal:         false,
						macAddress:      conjureMac("02-11", netutils.ParseIPSloppy(ep.ip)),
						providerAddress: providerAddress,
					}

					newHnsEndpoint, err = hns.createEndpoint(hnsEndpoint, hnsNetworkName)
					if err != nil {
						klog.ErrorS(err, "Remote endpoint creation failed", "windowsEndpoint", hnsEndpoint)
						continue
					}
				} else {

					hnsEndpoint := &windowsEndpoint{
						ip:         ep.ip,
						isLocal:    false,
						macAddress: ep.macAddress,
					}

					newHnsEndpoint, err = hns.createEndpoint(hnsEndpoint, hnsNetworkName)
					if err != nil {
						klog.ErrorS(err, "Remote endpoint creation failed")
						continue
					}
				}
			}

			// For Overlay networks 'SourceVIP' on an Load balancer Policy can either be chosen as
			// a) Source VIP configured on kube-proxy (or)
			// b) Node IP of the current node
			//
			// For L2Bridge network the Source VIP is always the NodeIP of the current node and the same
			// would be configured on kube-proxy as SourceVIP
			//
			// The logic for choosing the SourceVIP in Overlay networks is based on the backend windowsEndpoint:
			// a) Endpoints are any IP's outside the cluster ==> Choose NodeIP as the SourceVIP
			// b) Endpoints are IP addresses of a remote node => Choose NodeIP as the SourceVIP
			// c) Everything else (Local POD's, Remote POD's, Node IP of current node) ==> Choose the configured SourceVIP
			if strings.EqualFold(Proxier.network.networkType, NETWORK_TYPE_OVERLAY) && !ep.GetIsLocal() {
				providerAddress := Proxier.network.findRemoteSubnetProviderAddress(ep.IP())

				isNodeIP := (ep.IP() == providerAddress)
				isPublicIP := (len(providerAddress) == 0)
				klog.InfoS("Endpoint on overlay network", "ip", ep.IP(), "hnsNetworkName", hnsNetworkName, "isNodeIP", isNodeIP, "isPublicIP", isPublicIP)

				containsNodeIP = containsNodeIP || isNodeIP
				containsPublicIP = containsPublicIP || isPublicIP
			}

			// Save the hnsId for reference
			klog.V(1).InfoS("Hns endpoint resource", "windowsEndpoint", newHnsEndpoint)

			hnsEndpoints = append(hnsEndpoints, *newHnsEndpoint)
			if newHnsEndpoint.GetIsLocal() {
				hnsLocalEndpoints = append(hnsLocalEndpoints, *newHnsEndpoint)
			} else {
				// We only share the refCounts for remote windowsEndpoint
				ep.refCount = Proxier.endPointsRefCount.getRefCount(newHnsEndpoint.hnsID)
				*ep.refCount++
			}

			ep.hnsID = newHnsEndpoint.hnsID

			klog.V(3).InfoS("Endpoint resource found", "windowsEndpoint", ep)
		}

		klog.V(3).InfoS("Associated windowsEndpoint for service", "windowsEndpoint", hnsEndpoints, "serviceName", svcName)

		if len(svcInfo.hnsID) > 0 {
			// This should not happen
			klog.InfoS("Load Balancer already exists -- Debug ", "hnsID", svcInfo.hnsID)
		}

		if len(hnsEndpoints) == 0 {
			klog.ErrorS(nil, "Endpoint information not available for service, not applying any policy", "serviceName", svcName)
			continue
		}

		klog.V(4).InfoS("Trying to apply Policies for service", "serviceInfo", svcInfo)
		var hnsLoadBalancer *loadBalancerInfo
		var sourceVip = Proxier.sourceVip
		if containsPublicIP || containsNodeIP {
			sourceVip = Proxier.nodeIP.String()
		}

		sessionAffinityClientIP := svcInfo.SessionAffinityType() == v1.ServiceAffinityClientIP
		if sessionAffinityClientIP && !Proxier.supportedFeatures.SessionAffinity {
			klog.InfoS("Session Affinity is not supported on this version of Windows")
		}

		hnsLoadBalancer, err := hns.getLoadBalancer(
			hnsEndpoints,
			loadBalancerFlags{isDSR: Proxier.isDSR, isIPv6: Proxier.isIPv6Mode, sessionAffinity: sessionAffinityClientIP},
			sourceVip,
			svcInfo.ClusterIP().String(),
			Enum(svcInfo.Protocol()),
			uint16(svcInfo.targetPort),
			uint16(svcInfo.Port()),
		)
		if err != nil {
			klog.ErrorS(err, "Policy creation failed")
			continue
		}

		svcInfo.hnsID = hnsLoadBalancer.hnsID
		klog.V(3).InfoS("Hns LoadBalancer resource created for cluster ip resources", "clusterIP", svcInfo.ClusterIP(), "hnsID", hnsLoadBalancer.hnsID)

		// If nodePort is specified, user should be able to use nodeIP:nodePort to reach the backend windowsEndpoint
		if svcInfo.NodePort() > 0 {
			// If the preserve-destination service annotation is present, we will disable routing mesh for NodePort.
			// This means that health services can use Node Port without falsely getting results from a different node.
			nodePortEndpoints := hnsEndpoints
			if svcInfo.preserveDIP || svcInfo.localTrafficDSR {
				nodePortEndpoints = hnsLocalEndpoints
			}

			if len(nodePortEndpoints) > 0 {
				hnsLoadBalancer, err := hns.getLoadBalancer(
					nodePortEndpoints,
					loadBalancerFlags{isDSR: svcInfo.localTrafficDSR, localRoutedVIP: true, sessionAffinity: sessionAffinityClientIP, isIPv6: Proxier.isIPv6Mode},
					sourceVip,
					"",
					Enum(svcInfo.Protocol()),
					uint16(svcInfo.targetPort),
					uint16(svcInfo.NodePort()),
				)
				if err != nil {
					klog.ErrorS(err, "Policy creation failed")
					continue
				}

				svcInfo.nodePorthnsID = hnsLoadBalancer.hnsID
				klog.V(3).InfoS("Hns LoadBalancer resource created for nodePort resources", "clusterIP", svcInfo.ClusterIP(), "nodeport", svcInfo.NodePort(), "hnsID", hnsLoadBalancer.hnsID)
			} else {
				klog.V(3).InfoS("Skipped creating Hns LoadBalancer for nodePort resources", "clusterIP", svcInfo.ClusterIP(), "nodeport", svcInfo.NodePort(), "hnsID", hnsLoadBalancer.hnsID)
			}
		}

		// Create a Load Balancer Policy for each external IP
		for _, externalIP := range svcInfo.externalIPs {
			// Disable routing mesh if ExternalTrafficPolicy is set to local
			externalIPEndpoints := hnsEndpoints
			if svcInfo.localTrafficDSR {
				externalIPEndpoints = hnsLocalEndpoints
			}

			if len(externalIPEndpoints) > 0 {
				// Try loading existing policies, if already available
				hnsLoadBalancer, err = hns.getLoadBalancer(
					externalIPEndpoints,
					loadBalancerFlags{isDSR: svcInfo.localTrafficDSR, sessionAffinity: sessionAffinityClientIP, isIPv6: Proxier.isIPv6Mode},
					sourceVip,
					externalIP.ip,
					Enum(svcInfo.Protocol()),
					uint16(svcInfo.targetPort),
					uint16(svcInfo.Port()),
				)
				if err != nil {
					klog.ErrorS(err, "Policy creation failed")
					continue
				}
				externalIP.hnsID = hnsLoadBalancer.hnsID
				klog.V(3).InfoS("Hns LoadBalancer resource created for externalIP resources", "externalIP", externalIP, "hnsID", hnsLoadBalancer.hnsID)
			} else {
				klog.V(3).InfoS("Skipped creating Hns LoadBalancer for externalIP resources", "externalIP", externalIP, "hnsID", hnsLoadBalancer.hnsID)
			}
		}
		// Create a Load Balancer Policy for each loadbalancer ingress
		for _, lbIngressIP := range svcInfo.loadBalancerIngressIPs {
			// Try loading existing policies, if already available
			lbIngressEndpoints := hnsEndpoints
			if svcInfo.preserveDIP || svcInfo.localTrafficDSR {
				lbIngressEndpoints = hnsLocalEndpoints
			}

			if len(lbIngressEndpoints) > 0 {
				hnsLoadBalancer, err := hns.getLoadBalancer(
					lbIngressEndpoints,
					loadBalancerFlags{isDSR: svcInfo.preserveDIP || svcInfo.localTrafficDSR, useMUX: svcInfo.preserveDIP, preserveDIP: svcInfo.preserveDIP, sessionAffinity: sessionAffinityClientIP, isIPv6: Proxier.isIPv6Mode},
					sourceVip,
					lbIngressIP.ip,
					Enum(svcInfo.Protocol()),
					uint16(svcInfo.targetPort),
					uint16(svcInfo.Port()),
				)
				if err != nil {
					klog.ErrorS(err, "Policy creation failed")
					continue
				}
				lbIngressIP.hnsID = hnsLoadBalancer.hnsID
				klog.V(3).InfoS("Hns LoadBalancer resource created for loadBalancer Ingress resources", "lbIngressIP", lbIngressIP)
			} else {
				klog.V(3).InfoS("Skipped creating Hns LoadBalancer for loadBalancer Ingress resources", "lbIngressIP", lbIngressIP)
			}

		}
		svcInfo.policyApplied = true
		klog.V(2).InfoS("Policy successfully applied for service", "serviceInfo", svcInfo)
	}

	// TODO Jay:avoiding pkg/proxy imports... Commented out more healthServer stuff... going to kill it or keep it ?

	//	if Proxier.healthzServer != nil {
	//		Proxier.healthzServer.Updated()
	//	}
	//metrics.SyncProxyRulesLastTimestamp.SetToCurrentTime()

	// Update service healthchecks.  The windowsEndpoint list might include services that are
	// not "OnlyLocal", but the services list will not, and the serviceHealthServer
	// will just drop those windowsEndpoint.
	//	if err := Proxier.serviceHealthServer.SyncServices(serviceUpdateResult.HCServiceNodePorts); err != nil {
	//		klog.ErrorS(err, "Error syncing healthcheck services")
	//	}
	//	if err := Proxier.serviceHealthServer.SyncEndpoints(endpointUpdateResult.HCEndpointsLocalIPSize); err != nil {
	//		klog.ErrorS(err, "Error syncing healthcheck windowsEndpoint")
	//	}

	// Finish housekeeping.
	// TODO: these could be made more consistent.
	for _, svcIP := range staleServices.UnsortedList() {
		// TODO : Check if this is required to cleanup stale services here
		klog.V(5).InfoS("Pending delete stale service IP connections", "IP", svcIP)
	}

	// remove stale endpoint refcount entries
	for hnsID, referenceCount := range Proxier.endPointsRefCount {
		if *referenceCount <= 0 {
			delete(Proxier.endPointsRefCount, hnsID)
		}
	}
}
