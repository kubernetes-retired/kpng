package iptables

/*
Copyright 2019 The Kubernetes Authors.

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

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

// EndpointsCache is used as a cache of EndpointSlice information.
type EndpointsCache struct {
	// trackerByServiceMap is the basis of this cache. It contains endpoint
	// slice trackers grouped by service name and endpoint slice name. The first
	// key represents a namespaced service name while the second key represents
	// an endpoint slice name. Since endpoints can move between slices, we
	// require slice specific caching to prevent endpoints being removed from
	// the cache when they may have just moved to a different slice.
	trackerByServiceMap map[types.NamespacedName]*endpointsInfoByName
	hostname            string
	ipFamily            v1.IPFamily
	recorder            events.EventRecorder
}

// endpointsInfoByName groups endpointInfo by the names of the
// corresponding Endpoint.
type endpointsInfoByName map[string]*localnetv1.Endpoint

// NewEndpointsCache initializes an EndpointCache.
func NewEndpointsCache(hostname string, ipFamily v1.IPFamily, recorder events.EventRecorder) *EndpointsCache {
	return &EndpointsCache{
		trackerByServiceMap: map[types.NamespacedName]*endpointsInfoByName{},
		hostname:            hostname,
		ipFamily:            ipFamily,
		recorder:            recorder,
	}
}

// updatePending updates a pending slice in the cache.
func (cache *EndpointsCache) updatePending(svcKey types.NamespacedName, key string, endpoint *localnetv1.Endpoint) bool {
	var esInfoMap *endpointsInfoByName
	var ok bool
	if esInfoMap, ok = cache.trackerByServiceMap[svcKey]; !ok {
		esInfoMap = &endpointsInfoByName{}
		cache.trackerByServiceMap[svcKey] = esInfoMap
	}
	(*esInfoMap)[key] = endpoint
	return true
}

func (cache *EndpointsCache) isLocal(hostname string) bool {
	return len(cache.hostname) > 0 && hostname == cache.hostname
}
