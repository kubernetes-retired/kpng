/*
Copyright 2017 The Kubernetes Authors.

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

package iptables

import (
	"time"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/events"

	//"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

var supportedEndpointSliceAddressTypes = sets.NewString(
	string(discovery.AddressTypeIPv4),
	string(discovery.AddressTypeIPv6),
)

//var EndpointChangesPending = metrics.NewGauge(
//	&metrics.GaugeOpts{
//		Subsystem:      kubeProxySubsystem,
//		Name:           "sync_proxy_rules_endpoint_changes_pending",
//		Help:           "Pending proxy rules Endpoint changes",
//		StabilityLevel: metrics.ALPHA,
//	},
//)

// EndpointChangesTotal is the number of endpoint changes that the proxy
// has seen.
//var EndpointChangesTotal = metrics.NewCounter(
//	&metrics.CounterOpts{
//		Subsystem:      kubeProxySubsystem,
//		Name:           "sync_proxy_rules_endpoint_changes_total",
//		Help:           "Cumulative proxy rules Endpoint changes",
//		StabilityLevel: metrics.ALPHA,
//	},
//)

const kubeProxySubsystem = "kubeproxy"

// EndpointChangeTracker carries state about uncommitted changes to an arbitrary number of
// Endpoints, keyed by their namespace and name.
type EndpointChangeTracker struct {
	// hostname is the host where kube-proxy is running.
	hostname string
	// items maps a service to is endpointsChange.
	// items map[types.NamespacedName]*endpointsChange
	// makeEndpointInfo allows proxier to inject customized information when processing endpoint.
	// makeEndpointInfo          makeEndpointFunc
	// processEndpointsMapChange processEndpointsMapChangeFunc
	// endpointsCache holds a simplified version of endpoint slices.
	endpointsCache *EndpointsCache
	// ipfamily identify the ip family on which the tracker is operating on
	ipFamily v1.IPFamily
	recorder events.EventRecorder
	// Map from the Endpoints namespaced-name to the times of the triggers that caused the endpoints
	// object to change. Used to calculate the network-programming-latency.
	lastChangeTriggerTimes map[types.NamespacedName][]time.Time
	// record the time when the endpointChangeTracker was created so we can ignore the endpoints
	// that were generated before, because we can't estimate the network-programming-latency on those.
	// This is specially problematic on restarts, because we process all the endpoints that may have been
	// created hours or days before.
	trackerStartTime time.Time
}

// NewEndpointChangeTracker initializes an EndpointsChangeMap
func NewEndpointChangeTracker(hostname string, ipFamily v1.IPFamily, recorder events.EventRecorder) *EndpointChangeTracker {
	return &EndpointChangeTracker{
		hostname: hostname,
		// items:                  make(map[types.NamespacedName]*endpointsChange),
		ipFamily:               ipFamily,
		recorder:               recorder,
		lastChangeTriggerTimes: make(map[types.NamespacedName][]time.Time),
		trackerStartTime:       time.Now(),
		// processEndpointsMapChange: processEndpointsMapChange,
		endpointsCache: NewEndpointsCache(hostname, ipFamily, recorder),
	}
}

func (ect *EndpointChangeTracker) EndpointUpdate(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	namespacedName := types.NamespacedName{Name: serviceName, Namespace: namespace}
	EndpointChangesTotal.Inc()
	ect.endpointsCache.updatePending(namespacedName, key, endpoint)
}

// checkoutTriggerTimes applies the locally cached trigger times to a map of
// trigger times that have been passed in and empties the local cache.
func (ect *EndpointChangeTracker) checkoutTriggerTimes(lastChangeTriggerTimes *map[types.NamespacedName][]time.Time) {
	for k, v := range ect.lastChangeTriggerTimes {
		prev, ok := (*lastChangeTriggerTimes)[k]
		if !ok {
			(*lastChangeTriggerTimes)[k] = v
		} else {
			(*lastChangeTriggerTimes)[k] = append(prev, v...)
		}
	}
	ect.lastChangeTriggerTimes = make(map[types.NamespacedName][]time.Time)
}

// getLastChangeTriggerTime returns the time.Time value of the
// EndpointsLastChangeTriggerTime annotation stored in the given endpoints
// object or the "zero" time if the annotation wasn't set or was set
// incorrectly.
func getLastChangeTriggerTime(annotations map[string]string) time.Time {
	// TODO(#81360): ignore case when Endpoint is deleted.
	if _, ok := annotations[v1.EndpointsLastChangeTriggerTime]; !ok {
		// It's possible that the Endpoints object won't have the
		// EndpointsLastChangeTriggerTime annotation set. In that case return
		// the 'zero value', which is ignored in the upstream code.
		return time.Time{}
	}
	val, err := time.Parse(time.RFC3339Nano, annotations[v1.EndpointsLastChangeTriggerTime])
	if err != nil {
		klog.Warningf("Error while parsing EndpointsLastChangeTriggerTimeAnnotation: '%s'. Error is %v",
			annotations[v1.EndpointsLastChangeTriggerTime], err)
		// In case of error val = time.Zero, which is ignored in the upstream code.
	}
	return val
}

// UpdateEndpointMapResult is the updated results after applying endpoints changes.
type UpdateEndpointMapResult struct {
	// HCEndpointsLocalIPSize maps an endpoints name to the length of its local IPs.
	HCEndpointsLocalIPSize map[types.NamespacedName]int
	// StaleEndpoints identifies if an endpoints service pair is stale.
	StaleEndpoints []ServiceEndpoint
	// StaleServiceNames identifies if a service is stale.
	StaleServiceNames []ServicePortName
	// List of the trigger times for all endpoints objects that changed. It's used to export the
	// network programming latency.
	// NOTE(oxddr): this can be simplified to []time.Time if memory consumption becomes an issue.
	LastChangeTriggerTimes map[types.NamespacedName][]time.Time
}

// Update updates endpointsMap base on the given changes.
func (em EndpointsMap) Update(changes *EndpointChangeTracker) (result UpdateEndpointMapResult) {
	result.StaleEndpoints = make([]ServiceEndpoint, 0)
	result.StaleServiceNames = make([]ServicePortName, 0)
	result.LastChangeTriggerTimes = make(map[types.NamespacedName][]time.Time)
	em.apply(
		changes, &result.StaleEndpoints, &result.StaleServiceNames, &result.LastChangeTriggerTimes)
	// TODO: If this will appear to be computationally expensive, consider
	// computing this incrementally similarly to endpointsMap.
	result.HCEndpointsLocalIPSize = make(map[types.NamespacedName]int)
	localIPs := em.getLocalReadyEndpointIPs()
	for nsn, ips := range localIPs {
		result.HCEndpointsLocalIPSize[nsn] = len(ips)
	}
	changes.endpointsCache.trackerByServiceMap = map[types.NamespacedName]*endpointsInfoByName{}
	return result
}

// EndpointsMap maps a service name to a list of all its Endpoints.
type EndpointsMap map[types.NamespacedName]*endpointsInfoByName

// apply the changes to EndpointsMap and updates stale endpoints and service-endpoints pair. The `staleEndpoints` argument
// is passed in to store the stale udp endpoints and `staleServiceNames` argument is passed in to store the stale udp service.
// The changes map is cleared after applying them.
// In addition it returns (via argument) and resets the lastChangeTriggerTimes for all endpoints
// that were changed and will result in syncing the proxy rules.
// apply triggers processEndpointsMapChange on every change.
func (em EndpointsMap) apply(ect *EndpointChangeTracker, staleEndpoints *[]ServiceEndpoint,
	staleServiceNames *[]ServicePortName, lastChangeTriggerTimes *map[types.NamespacedName][]time.Time) {
	if ect == nil {
		return
	}
	em.merge(ect.endpointsCache.trackerByServiceMap)
	// TODO: CHECK detect stale later
	// detectStaleConnections(change.previous, change.current, staleEndpoints, staleServiceNames)
	// }
	ect.checkoutTriggerTimes(lastChangeTriggerTimes)
}

// Merge ensures that the current EndpointsMap contains all <service, endpoints> pairs from the EndpointsMap passed in.
func (em EndpointsMap) merge(other map[types.NamespacedName]*endpointsInfoByName) {
	for svcPortName, epInfo := range other {
		for name, ep := range *(epInfo) {
			if ep == nil {
				//TODO : if servicemap contains UDP port , then save the namespace, name ,protocol and epip
				//  in cache as stale
				delete(*(em[svcPortName]), name)
				if len(*em[svcPortName]) <= 0 {
					delete(em, svcPortName)
				}
				continue
			}
			var epInfoMap *endpointsInfoByName
			var ok bool
			if epInfoMap, ok = em[svcPortName]; !ok {
				epInfoMap = &endpointsInfoByName{}
				em[svcPortName] = epInfoMap
			}
			(*(epInfoMap))[name] = ep
		}
	}
}

// GetLocalEndpointIPs returns endpoints IPs if given endpoint is local - local means the endpoint is running in same host as kube-proxy.
func (em EndpointsMap) getLocalReadyEndpointIPs() map[types.NamespacedName]sets.String {
	localIPs := make(map[types.NamespacedName]sets.String)
	for svcPortName, epList := range em {
		for _, ep := range *epList {
			// Only add ready endpoints for health checking. Terminating endpoints may still serve traffic
			// but the health check signal should fail if there are only terminating endpoints on a node.
			//TODO: CHECK no endpoint.Topology and endpoint.Conditions Endpointslicecache.go
			// if !ep.IsReady() {
			// 	continue
			// }

			if ep.Local {
				nsn := svcPortName
				if localIPs[nsn] == nil {
					localIPs[nsn] = sets.NewString()
				}
				localIPs[nsn].Insert(ep.IPs.All()...)
			}
		}
	}
	return localIPs
}

// TODO:detectStaleConnections modifies <staleEndpoints> and <staleServices> with detected stale connections. <staleServiceNames>
// is used to store stale udp service in order to clear udp conntrack later.
// func detectStaleConnections(oldEndpointsMap, newEndpointsMap EndpointsMap, staleEndpoints *[]ServiceEndpoint, staleServiceNames *[]ServicePortName) {
// 	for svcPortName, epList := range oldEndpointsMap {
// 		if svcPortName.Protocol != v1.ProtocolUDP {
// 			continue
// 		}

// 		for _, ep := range epList {
// 			stale := true
// 			for i := range newEndpointsMap[svcPortName] {
// 				if newEndpointsMap[svcPortName][i].Equal(ep) {
// 					stale = false
// 					break
// 				}
// 			}
// 			if stale {
// 				klog.V(4).Infof("Stale endpoint %v -> %v", svcPortName, ep.String())
// 				*staleEndpoints = append(*staleEndpoints, ServiceEndpoint{Endpoint: ep.String(), ServicePortName: svcPortName})
// 			}
// 		}
// 	}

// 	for svcPortName, epList := range newEndpointsMap {
// 		if svcPortName.Protocol != v1.ProtocolUDP {
// 			continue
// 		}

// 		// For udp service, if its backend changes from 0 to non-0. There may exist a conntrack entry that could blackhole traffic to the service.
// 		if len(epList) > 0 && len(oldEndpointsMap[svcPortName]) == 0 {
// 			*staleServiceNames = append(*staleServiceNames, svcPortName)
// 		}
// 	}
// }
