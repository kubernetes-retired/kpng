/*
Copyright 2021 The Kubernetes Authors.

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

package userspace

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"sigs.k8s.io/kpng/api/localv1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	stringslices "k8s.io/utils/strings/slices"
)

var (
	ErrMissingServiceEntry = errors.New("missing service entry")
	ErrMissingEndpoints    = errors.New("missing endpoints")
)

type affinityState struct {
	clientIP string
	//clientProtocol  api.Protocol //not yet used
	//sessionCookie   string       //not yet used
	endpoint string
	lastUsed time.Time
}

type affinityPolicy struct {
	affinityClientIP bool
	affinityMap      map[string]*affinityState // map client IP -> affinity info
	ttlSeconds       int
}

// LoadBalancerRR is a round-robin load balancer.
type LoadBalancerRR struct {
	lock     sync.RWMutex
	services map[ServicePortName]*balancerState
}

// Ensure this implements LoadBalancer.
var _ LoadBalancer = &LoadBalancerRR{}

type balancerState struct {
	endpoints []string // a list of "ip:port" style strings
	index     int      // current index into endpoints
	affinity  *affinityPolicy
}

func newAffinityPolicy(affinityClientIP *localv1.ClientIPAffinity, ttlSeconds int) *affinityPolicy {
	return &affinityPolicy{
		affinityClientIP: affinityClientIP != nil,
		affinityMap:      make(map[string]*affinityState),
		ttlSeconds:       ttlSeconds,
	}
}

// NewLoadBalancerRR returns a new LoadBalancerRR.
func NewLoadBalancerRR() *LoadBalancerRR {
	return &LoadBalancerRR{
		services: map[ServicePortName]*balancerState{},
	}
}

func (lb *LoadBalancerRR) NewService(svcPort ServicePortName, affinityClientIP *localv1.ClientIPAffinity, ttlSeconds int) error {
	klog.V(4).InfoS("LoadBalancerRR NewService", "servicePortName", svcPort)
	lb.lock.Lock()
	defer lb.lock.Unlock()
	lb.newServiceInternal(svcPort, affinityClientIP, ttlSeconds)
	return nil
}

// This assumes that lb.lock is already held.
func (lb *LoadBalancerRR) newServiceInternal(svcPort ServicePortName, affinityClientIP *localv1.ClientIPAffinity, ttlSeconds int) *balancerState {
	if ttlSeconds == 0 {
		ttlSeconds = int(v1.DefaultClientIPServiceAffinitySeconds) //default to 3 hours if not specified.  Should 0 be unlimited instead????
	}

	if _, exists := lb.services[svcPort]; !exists {
		lb.services[svcPort] = &balancerState{affinity: newAffinityPolicy(affinityClientIP, ttlSeconds)}
		klog.V(4).InfoS("LoadBalancerRR service did not exist, created", "servicePortName", svcPort)
	} else if affinityClientIP != nil {
		lb.services[svcPort].affinity.affinityClientIP = true
	}
	return lb.services[svcPort]
}

func (lb *LoadBalancerRR) DeleteService(svcPort ServicePortName) {
	klog.V(4).InfoS("LoadBalancerRR DeleteService", "servicePortName", svcPort)
	lb.lock.Lock()
	defer lb.lock.Unlock()
	delete(lb.services, svcPort)
}

// NextEndpoint returns a service endpoint.
// The service endpoint is chosen using the round-robin algorithm.
func (lb *LoadBalancerRR) NextEndpoint(svcPort ServicePortName, srcAddr net.Addr, sessionAffinityReset bool) (string, error) {
	// Coarse locking is simple.  We can get more fine-grained if/when we
	// can prove it matters.
	lb.lock.Lock()
	defer lb.lock.Unlock()

	state, exists := lb.services[svcPort]
	if !exists || state == nil {
		return "", ErrMissingServiceEntry
	}
	if len(state.endpoints) == 0 {
		return "", ErrMissingEndpoints
	}
	klog.V(4).InfoS("NextEndpoint for service", "servicePortName", svcPort, "address", srcAddr, "endpoints", state.endpoints)

	var ipaddr string
	if state.affinity.affinityClientIP {
		// Caution: don't shadow ipaddr
		var err error
		ipaddr, _, err = net.SplitHostPort(srcAddr.String())
		if err != nil {
			return "", fmt.Errorf("malformed source address %q: %v", srcAddr.String(), err)
		}
		if !sessionAffinityReset {
			sessionAffinity, exists := state.affinity.affinityMap[ipaddr]
			if exists && int(time.Since(sessionAffinity.lastUsed).Seconds()) < state.affinity.ttlSeconds {
				// Affinity wins.
				endpoint := sessionAffinity.endpoint
				sessionAffinity.lastUsed = time.Now()
				klog.V(4).InfoS("NextEndpoint for service from IP with sessionAffinity", "servicePortName", svcPort, "IP", ipaddr, "sessionAffinity", sessionAffinity, "endpoint", endpoint)
				return endpoint, nil
			}
		}
	}
	// Take the next endpoint.
	endpoint := state.endpoints[state.index]
	state.index = (state.index + 1) % len(state.endpoints)

	if state.affinity.affinityClientIP {
		var affinity *affinityState
		affinity = state.affinity.affinityMap[ipaddr]
		if affinity == nil {
			affinity = new(affinityState) //&affinityState{ipaddr, "TCP", "", endpoint, time.Now()}
			state.affinity.affinityMap[ipaddr] = affinity
		}
		affinity.lastUsed = time.Now()
		affinity.endpoint = endpoint
		affinity.clientIP = ipaddr
		klog.V(4).InfoS("Updated affinity key", "IP", ipaddr, "affinityState", state.affinity.affinityMap[ipaddr])
	}

	return endpoint, nil
}

// Remove any session affinity records associated to a particular endpoint (for example when a pod goes down).
func removeSessionAffinityByEndpoint(state *balancerState, svcPort ServicePortName, endpoint string) {
	for _, affinity := range state.affinity.affinityMap {
		if affinity.endpoint == endpoint {
			klog.V(4).InfoS("Removing client from affinityMap for service", "endpoint", affinity.endpoint, "servicePortName", svcPort)
			delete(state.affinity.affinityMap, affinity.clientIP)
		}
	}
}

// Loop through the valid endpoints and then the endpoints associated with the Load Balancer.
// Then remove any session affinity records that are not in both lists.
// This assumes the lb.lock is held.
func (lb *LoadBalancerRR) updateAffinityMap(svcPort ServicePortName, newEndpoints []string) {
	allEndpoints := map[string]int{}
	for _, newEndpoint := range newEndpoints {
		allEndpoints[newEndpoint] = 1
	}
	state, exists := lb.services[svcPort]
	if !exists {
		return
	}
	for _, existingEndpoint := range state.endpoints {
		allEndpoints[existingEndpoint] = allEndpoints[existingEndpoint] + 1
	}
	for mKey, mVal := range allEndpoints {
		if mVal == 1 {
			klog.V(2).InfoS("Delete endpoint for service", "endpoint", mKey, "servicePortName", svcPort)
			removeSessionAffinityByEndpoint(state, svcPort, mKey)
		}
	}
}

func (lb *LoadBalancerRR) OnEndpointsAdd(ep *localv1.Endpoint, svc *localv1.Service) {
	portsToEndpoints := buildPortsToEndpointsMap(ep, svc)

	lb.lock.Lock()
	defer lb.lock.Unlock()

	for portname := range portsToEndpoints {
		svcPort := ServicePortName{NamespacedName: types.NamespacedName{Namespace: svc.GetNamespace(), Name: svc.GetName()}, Port: portname}
		state, _ := lb.services[svcPort]

		newEndpoints := portsToEndpoints[portname]
		if state != nil {
			newEndpoints = append(newEndpoints, state.endpoints...)
		}

		klog.V(1).InfoS("LoadBalancerRR: Setting endpoints for service", "servicePortName", svcPort, "endpoints", newEndpoints)
		lb.updateAffinityMap(svcPort, newEndpoints)
		// OnEndpointsUpdate can be called without NewService being called externally.
		// To be safe we will call it here.  A new service will only be created
		// if one does not already exist.  The affinity will be updated
		// later, once NewService is called.
		state = lb.newServiceInternal(svcPort, svc.GetClientIP(), 0)
		state.endpoints = ShuffleStrings(newEndpoints)

		// Reset the round-robin index.
		state.index = 0
	}

}

func (lb *LoadBalancerRR) OnEndpointsDelete(ep *localv1.Endpoint, svc *localv1.Service) {
	portsToEndpoints := buildPortsToEndpointsMap(ep, svc)

	lb.lock.Lock()
	defer lb.lock.Unlock()

	for portname := range portsToEndpoints {
		svcPort := ServicePortName{NamespacedName: types.NamespacedName{Namespace: svc.GetNamespace(), Name: svc.GetName()}, Port: portname}
		state, _ := lb.services[svcPort]

		if state == nil { // empty services endpoint
			continue
		}

		newEndpoints := []string{}
		for _, endpoint := range state.endpoints {
			if stringslices.Contains(portsToEndpoints[portname], endpoint) {
				continue
			}
			newEndpoints = append(newEndpoints, endpoint)
		}

		klog.V(2).InfoS("LoadBalancerRR: Removing endpoints service", "servicePortName", svcPort)
		lb.updateAffinityMap(svcPort, newEndpoints)
		// OnEndpointsUpdate can be called without NewService being called externally.
		// To be safe we will call it here.  A new service will only be created
		// if one does not already exist.  The affinity will be updated
		// later, once NewService is called.
		state = lb.newServiceInternal(svcPort, svc.GetClientIP(), 0)
		state.endpoints = ShuffleStrings(newEndpoints)
		// Reset the round-robin index.
		state.index = 0
	}
}

func (lb *LoadBalancerRR) OnEndpointsSynced() {
}

// Tests whether two slices are equivalent.  This sorts both slices in-place.
func slicesEquiv(lhs, rhs []string) bool {
	if len(lhs) != len(rhs) {
		return false
	}
	sort.Strings(lhs)
	sort.Strings(rhs)
	return stringslices.Equal(lhs, rhs)
}

func (lb *LoadBalancerRR) CleanupStaleStickySessions(svcPort ServicePortName) {
	lb.lock.Lock()
	defer lb.lock.Unlock()

	state, exists := lb.services[svcPort]
	if !exists {
		return
	}
	for ip, affinity := range state.affinity.affinityMap {
		if int(time.Since(affinity.lastUsed).Seconds()) >= state.affinity.ttlSeconds {
			klog.V(4).InfoS("Removing client from affinityMap for service", "IP", affinity.clientIP, "servicePortName", svcPort)
			delete(state.affinity.affinityMap, ip)
		}
	}
}
