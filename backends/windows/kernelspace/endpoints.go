//go:build windows
// +build windows

/*
Copyright 2018-2022 The Kubernetes Authors.

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
	"net"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/events"
	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"

	localv1 "sigs.k8s.io/kpng/api/localv1"
)

// internal struct for endpoints information
type endpointsInfo struct {
	ip              string
	port            uint16
	isLocal         bool
	macAddress      string
	hnsID           string
	refCount        *uint16
	providerAddress string
	hns             HCNUtils
	name            string

	// conditions
	ready       bool
	serving     bool
	terminating bool
}

// String is part of proxy.Endpoint interface.
func (info *endpointsInfo) String() string {
	return net.JoinHostPort(info.ip, strconv.Itoa(int(info.port)))
}

// GetIsLocal is part of proxy.Endpoint interface.
func (info *endpointsInfo) GetIsLocal() bool {
	return info.isLocal
}

// IsReady returns true if an endpoint is ready and not terminating.
func (info *endpointsInfo) IsReady() bool {
	return info.ready
}

// IsServing returns true if an endpoint is ready, regardless of it's terminating state.
func (info *endpointsInfo) IsServing() bool {
	return info.serving
}

// IsTerminating returns true if an endpoint is terminating.
func (info *endpointsInfo) IsTerminating() bool {
	return info.terminating
}

// GetZoneHint returns the zone hint for the endpoint.
func (info *endpointsInfo) GetZoneHints() sets.String {
	return sets.String{}
}

// IP returns just the IP part of the endpoint, it's a part of proxy.Endpoint interface.
func (info *endpointsInfo) IP() string {
	return info.ip
}

// Port returns just the Port part of the endpoint.
func (info *endpointsInfo) Port() (int, error) {
	return int(info.port), nil
}

// Equal is part of proxy.Endpoint interface.
func (info *endpointsInfo) Equal(other Endpoint) bool {
	return info.String() == other.String() && info.GetIsLocal() == other.GetIsLocal()
}

// GetNodeName returns the NodeName for this endpoint.
func (info *endpointsInfo) GetNodeName() string {
	return ""
}

// GetZone returns the Zone for this endpoint.
func (info *endpointsInfo) GetZone() string {
	return ""
}

func (info *endpointsInfo) GetTopology() map[string]string {
	return map[string]string{}
}

func (info *endpointsInfo) Cleanup() {
	klog.V(3).InfoS("Endpoint cleanup", "endpointsInfo", info)
	if !info.GetIsLocal() && info.refCount != nil {
		*info.refCount--

		// Remove the remote hns endpoint, if no service is referring it
		// Never delete a Local Endpoint. Local Endpoints are already created by other entities.
		// Remove only remote endpoints created by this service
		if *info.refCount <= 0 && !info.GetIsLocal() {
			klog.V(4).InfoS("Removing endpoints, since no one is referencing it", "endpoint", info)
			err := info.hns.deleteEndpoint(info.hnsID)
			if err == nil {
				info.hnsID = ""
			} else {
				klog.ErrorS(err, "Endpoint deletion failed", "ip", info.IP())
			}
		}

		info.refCount = nil
	}
}

var supportedEndpointSliceAddressTypes = sets.NewString(
	string(discovery.AddressTypeIPv4),
	string(discovery.AddressTypeIPv6),
)

const kubeProxySubsystem = "kubeproxy"

var EndpointChangesTotal = metrics.NewCounter(
	&metrics.CounterOpts{
		Subsystem:      kubeProxySubsystem,
		Name:           "sync_proxy_rules_endpoint_changes_total",
		Help:           "Cumulative proxy rules Endpoint changes",
		StabilityLevel: metrics.ALPHA,
	},
)

// EndpointsMap maps a service name to a list of all its Endpoints.
type EndpointsMap map[types.NamespacedName]*endpointsInfoByName

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
	// Map from the Endpoints namespaced-name to the times of the triggers that caused the windowsEndpoint
	// object to change. Used to calculate the network-programming-latency.
	lastChangeTriggerTimes map[types.NamespacedName][]time.Time
	// record the time when the endpointChangeTracker was created so we can ignore the windowsEndpoint
	// that were generated before, because we can't estimate the network-programming-latency on those.
	// This is specially problematic on restarts, because we process all the windowsEndpoint that may have been
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

// how to convert windowsEndpoint from a incoming ??? localvnet1 Endpoint ?
func (ect *EndpointChangeTracker) EndpointUpdate(namespace, serviceName, key string, we *localv1.Endpoint) {
	// func (ect *EndpointChangeTracker) EndpointUpdate(namespace, serviceName, key string,  we *windowsEndpoint) {
	namespacedName := types.NamespacedName{Name: serviceName, Namespace: namespace}
	EndpointChangesTotal.Inc()

	// xxxx 2 -> recieve a kpng endpoint and store it

	// how to convert endpoint to a windowsEndpoint (we) ???
	ect.endpointsCache.updatePending(namespacedName, key, we)
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
// EndpointsLastChangeTriggerTime annotation stored in the given windowsEndpoint
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

// UpdateEndpointMapResult is the updated results after applying windowsEndpoint changes.
type UpdateEndpointMapResult struct {
	// HCEndpointsLocalIPSize maps an windowsEndpoint name to the length of its local IPs.
	HCEndpointsLocalIPSize map[types.NamespacedName]int
	// StaleEndpoints identifies if an windowsEndpoint service pair is stale.
	StaleEndpoints []ServiceEndpoint
	// StaleServiceNames identifies if a service is stale.
	StaleServiceNames []ServicePortName
	// List of the trigger times for all windowsEndpoint objects that changed. It's used to export the
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
	changes.endpointsCache.trackerByServiceMap = EndpointsMap{}
	return result
}

// apply the changes to EndpointsMap and updates stale windowsEndpoint and service-windowsEndpoint pair. The `staleEndpoints` argument
// is passed in to store the stale udp windowsEndpoint and `staleServiceNames` argument is passed in to store the stale udp service.
// The changes map is cleared after applying them.
// In addition it returns (via argument) and resets the lastChangeTriggerTimes for all windowsEndpoint
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

// Merge ensures that the current EndpointsMap contains all <service, windowsEndpoint> pairs from the EndpointsMap passed in.
func (em EndpointsMap) merge(other EndpointsMap) {
	for service, endpoints := range other {
		for hash, endpointEntry := range *(endpoints) {
			if endpointEntry == nil {
				//TODO : if servicemap contains UDP port , then save the namespace, name ,protocol and epip
				//  in cache as stale
				delete(*(em[service]), hash)
				if len(*em[service]) <= 0 {
					delete(em, service)
				}
				continue
			}

			var endpointMap *endpointsInfoByName
			var ok bool

			// Check if EndPointsMap exists, if not, create a fresh map
			if endpointMap, ok = em[service]; !ok {
				endpointMap = &endpointsInfoByName{}
				em[service] = endpointMap
			}
			(*(endpointMap))[hash] = endpointEntry
		}
	}
}

// GetLocalEndpointIPs returns windowsEndpoint IPs if given endpoint is local - local means the endpoint is running in same host as kube-proxy.
func (em EndpointsMap) getLocalReadyEndpointIPs() map[types.NamespacedName]sets.String {
	localIPs := make(map[types.NamespacedName]sets.String)
	for service, endpoints := range em {
		for _, endpointEntry := range *endpoints {
			// Only add ready windowsEndpoint for health checking. Terminating windowsEndpoint may still serve traffic
			// but the health check signal should fail if there are only terminating windowsEndpoint on a node.
			//TODO: CHECK no endpoint.Topology and endpoint.Conditions Endpointslicecache.go
			// if !ep.IsReady() {
			// 	continue
			// }

			if endpointEntry.Local {
				nsn := service
				if localIPs[nsn] == nil {
					localIPs[nsn] = sets.NewString()
				}
				// localIPs[nsn].Insert(endpointEntry.IPs.All()...)
				// removed this ever since got rid of the windowsEndpoint
				// 	localIPs[nsn].Insert(endpointEntry.ip)
				//  not sure if the first IP works here or not....
				localIPs[nsn].Insert(endpointEntry.IPs.All()[0])

			}
		}
	}
	return localIPs
}

// EndpointsCache is used as a cache of EndpointSlice information.
type EndpointsCache struct {
	// trackerByServiceMap is the basis of this cache. It contains endpoint
	// slice trackers grouped by service name and endpoint slice name. The first
	// key represents a namespaced service name while the second key represents
	// an endpoint slice name. Since windowsEndpoint can move between slices, we
	// require slice specific caching to prevent windowsEndpoint being removed from
	// the cache when they may have just moved to a different slice.
	trackerByServiceMap map[types.NamespacedName]*endpointsInfoByName
	hostname            string
	ipFamily            v1.IPFamily
	recorder            events.EventRecorder
}

// endpointsInfoByName groups endpointInfo by the names of the
// corresponding Endpoint.
// xxxxxxxxxxxxxxxxxxxxxxxx 4 compile error -----> windowsEndpoint
type endpointsInfoByName map[string]*localv1.Endpoint

// NewEndpointsCache initializes an EndpointCache.
func NewEndpointsCache(hostname string, ipFamily v1.IPFamily, recorder events.EventRecorder) *EndpointsCache {
	return &EndpointsCache{
		trackerByServiceMap: EndpointsMap{},
		hostname:            hostname,
		ipFamily:            ipFamily,
		recorder:            recorder,
	}
}

// xxxx 3 ---> now updatePending, write the localvnet1 endpoint
// updatePending updates a pending slice in the cache.
func (cache *EndpointsCache) updatePending(svcKey types.NamespacedName, key string, we *localv1.Endpoint) bool {
	var esInfoMap *endpointsInfoByName
	var ok bool
	if esInfoMap, ok = cache.trackerByServiceMap[svcKey]; !ok {
		esInfoMap = &endpointsInfoByName{}
		cache.trackerByServiceMap[svcKey] = esInfoMap
	}

	(*esInfoMap)[key] = we
	return true
}

func (cache *EndpointsCache) isLocal(hostname string) bool {
	return len(cache.hostname) > 0 && hostname == cache.hostname
}
