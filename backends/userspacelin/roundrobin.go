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

package userspacelin

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/kpng/api/localnetv1"

	"sigs.k8s.io/kpng/backends/iptables"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	klog "k8s.io/klog/v2"
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
	services map[iptables.ServicePortName]*balancerState
}

// Ensure this implements LoadBalancer.
var _ LoadBalancer = &LoadBalancerRR{}

type balancerState struct {
	endpoints []string // a list of "ip:port" style strings
	index     int      // current index into endpoints
	affinity  affinityPolicy
}

func newAffinityPolicy(affinityClientIP *localnetv1.ClientIPAffinity, ttlSeconds int) *affinityPolicy {
	return &affinityPolicy{
		affinityClientIP: affinityClientIP != nil,
		affinityMap:      make(map[string]*affinityState),
		ttlSeconds:       ttlSeconds,
	}
}

// NewLoadBalancerRR returns a new LoadBalancerRR.
func NewLoadBalancerRR() *LoadBalancerRR {
	return &LoadBalancerRR{
		services: map[iptables.ServicePortName]*balancerState{},
	}
}

func (lb *LoadBalancerRR) NewService(svcPort iptables.ServicePortName, affinityType *localnetv1.ClientIPAffinity, ttlSeconds int) error {
	klog.V(4).Infof("LoadBalancerRR NewService %q", svcPort)
	lb.lock.Lock()
	lb.lock.Unlock()
	lb.newServiceInternal(svcPort, affinityType, ttlSeconds)
	return nil
}

// This assumes that lb.lock is already held.
func (lb *LoadBalancerRR) newServiceInternal(svcPort iptables.ServicePortName, affinityClientIP *localnetv1.ClientIPAffinity, ttlSeconds int) *balancerState {
	if ttlSeconds == 0 {
		ttlSeconds = int(v1.DefaultClientIPServiceAffinitySeconds) //default to 3 hours if not specified.  Should 0 be unlimited instead????
	}

	if _, exists := lb.services[svcPort]; !exists {
		lb.services[svcPort] = &balancerState{affinity: *newAffinityPolicy(affinityClientIP, ttlSeconds)}
		klog.V(4).Infof("LoadBalancerRR service %q did not exist, created", svcPort)
	} else if affinityClientIP != nil {
		lb.services[svcPort].affinity.affinityClientIP = true
	}
	return lb.services[svcPort]
}

func (lb *LoadBalancerRR) DeleteService(svcPort iptables.ServicePortName) {
	klog.V(4).Infof("LoadBalancerRR DeleteService %q", svcPort)
	lb.lock.Lock()
	defer lb.lock.Unlock()
	delete(lb.services, svcPort)
}

// return true if this service is using some form of session affinity.
func isSessionAffinity(affinity *affinityPolicy) bool {
	// Should never be empty string, but checking for it to be safe.
	if affinity.affinityClientIP == false {
		return false
	}
	return true
}

// ServiceHasEndpoints checks whether a service entry has endpoints.
func (lb *LoadBalancerRR) ServiceHasEndpoints(svcPort iptables.ServicePortName) bool {
	lb.lock.RLock()
	defer lb.lock.RUnlock()
	state, exists := lb.services[svcPort]
	// TODO: while nothing ever assigns nil to the map, *some* of the code using the map
	// checks for it.  The code should all follow the same convention.
	return exists && state != nil && len(state.endpoints) > 0
}

// NextEndpoint returns a service endpoint.
// The service endpoint is chosen using the round-robin algorithm.
func (lb *LoadBalancerRR) NextEndpoint(svcPort iptables.ServicePortName, srcAddr net.Addr, sessionAffinityReset bool) (string, error) {
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
	klog.V(4).Infof("NextEndpoint for service %q, srcAddr=%v: endpoints: %+v", svcPort, srcAddr, state.endpoints)

	sessionAffinityEnabled := isSessionAffinity(&state.affinity)

	var ipaddr string
	if sessionAffinityEnabled {
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
				klog.V(4).Infof("NextEndpoint for service %q from IP %s with sessionAffinity %#v: %s", svcPort, ipaddr, sessionAffinity, endpoint)
				return endpoint, nil
			}
		}
	}
	// Take the next endpoint.
	endpoint := state.endpoints[state.index]
	state.index = (state.index + 1) % len(state.endpoints)

	if sessionAffinityEnabled {
		var affinity *affinityState
		affinity = state.affinity.affinityMap[ipaddr]
		if affinity == nil {
			affinity = new(affinityState) //&affinityState{ipaddr, "TCP", "", endpoint, time.Now()}
			state.affinity.affinityMap[ipaddr] = affinity
		}
		affinity.lastUsed = time.Now()
		affinity.endpoint = endpoint
		affinity.clientIP = ipaddr
		klog.V(4).Infof("Updated affinity key %s: %#v", ipaddr, state.affinity.affinityMap[ipaddr])
	}

	return endpoint, nil
}

// Remove any session affinity records associated to a particular endpoint (for example when a pod goes down).
func removeSessionAffinityByEndpoint(state *balancerState, svcPort iptables.ServicePortName, endpoint string) {
	for _, affinity := range state.affinity.affinityMap {
		if affinity.endpoint == endpoint {
			klog.V(4).Infof("Removing client: %s from affinityMap for service %q", affinity.endpoint, svcPort)
			delete(state.affinity.affinityMap, affinity.clientIP)
		}
	}
}

// Loop through the valid endpoints and then the endpoints associated with the Load Balancer.
// Then remove any session affinity records that are not in both lists.
// This assumes the lb.lock is held.
func (lb *LoadBalancerRR) removeStaleAffinity(svcPort iptables.ServicePortName, newEndpoints []string) {
	newEndpointsSet := sets.NewString()
	for _, newEndpoint := range newEndpoints {
		newEndpointsSet.Insert(newEndpoint)
	}

	state, exists := lb.services[svcPort]
	if !exists {
		return
	}
	for _, existingEndpoint := range state.endpoints {
		if !newEndpointsSet.Has(existingEndpoint) {
			klog.V(2).Infof("Delete endpoint %s for service %q", existingEndpoint, svcPort)
			removeSessionAffinityByEndpoint(state, svcPort, existingEndpoint)
		}
	}
}

func (lb *LoadBalancerRR) OnEndpointsAdd(ep *localnetv1.Endpoint, svc *localnetv1.Service) {
	portsToEndpoints := buildPortsToEndpointsMap(ep, svc)
	namespace := svc.Namespace
	name := svc.Name
	namespacedName := types.NamespacedName{Namespace: namespace, Name: name}

	lb.lock.Lock()
	defer lb.lock.Unlock()

	for portname := range portsToEndpoints {
		// OMG endpoints are named the same thing as their service so we can use this to find the service name
		// MEANWHILE endpointSlice has a LABEL that references the service
		svcPort := iptables.ServicePortName{NamespacedName: namespacedName, Port: portname}
		newEndpoints := portsToEndpoints[portname]
		state, exists := lb.services[svcPort]
		if state != nil {
			newEndpoints = append(newEndpoints, state.endpoints...)
		}
		if !exists || state == nil || len(newEndpoints) > 0 {
			klog.V(1).Infof("LoadBalancerRR: Setting endpoints for %s to %+v", svcPort, newEndpoints)
			// OnEndpointsAdd can be called without NewService being called externally.
			// To be safe we will call it here.  A new service will only be created
			// if one does not already exist.
			state = lb.newServiceInternal(svcPort, svc.GetClientIP(), 0)
			state.endpoints = ShuffleStrings(newEndpoints)
			// Reset the round-robin index.
			state.index = 0
		}
	}
}

//[]*v1.Endpoints, endpoint
// func (lb *LoadBalancerRR) OnEndpointsUpdate(oldEndpoints, endpoints *v1.Endpoints) {

// 	// part 1: make new endpoints as needed stuff...

// 	portsToEndpoints := buildPortsToEndp
// 	ointsMap(endpoints)
// 	registeredEndpoints := make(map[iptables.ServicePortName]bool)

// 	lb.lock.Lock()
// 	defer lb.lock.Unlock()

// 	// new stuff
// 	for portname := range portsToEndpoints {
// 		svcPort := iptables.ServicePortName{NamespacedName: types.NamespacedName{Namespace: endpoints.Namespace, Name: endpoints.Name}, Port: portname}
// 		newEndpoints := portsToEndpoints[portname]
// 		state, exists := lb.services[svcPort]

// 		curEndpoints := []string{}
// 		if state != nil {
// 			curEndpoints = state.endpoints
// 		}

// 		if !exists || state == nil || len(curEndpoints) != len(newEndpoints) || !slicesEquiv(copyStrings(curEndpoints), newEndpoints) {
// 			klog.V(1).Infof("LoadBalancerRR: Setting endpoints for %s to %+v", svcPort, newEndpoints)
// 			lb.removeStaleAffinity(svcPort, newEndpoints)
// 			// OnEndpointsUpdate can be called without NewService being called externally.
// 			// To be safe we will call it here.  A new service will only be created
// 			// if one does not already exist.  The affinity will be updated
// 			// later, once NewService is called.
// 			state = lb.newServiceInternal(svcPort, v1.ServiceAffinity(""), 0)
// 			state.endpoints = ShuffleStrings(newEndpoints)

// 			// Reset the round-robin index.
// 			state.index = 0
// 		}
// 		registeredEndpoints[svcPort] = true
// 	}

// 	// part 2: clean old stuff
// 	oldPortsToEndpoints := buildPortsToEndpointsMap(oldEndpoints)

// 	// clean old stuff
// 	// Now remove all endpoints missing from the update.
// 	for portname := range oldPortsToEndpoints {
// 		svcPort := iptables.ServicePortName{NamespacedName: types.NamespacedName{Namespace: oldEndpoints.Namespace, Name: oldEndpoints.Name}, Port: portname}
// 		if _, exists := registeredEndpoints[svcPort]; !exists {
// 			lb.resetService(svcPort)
// 		}
// 	}
// }

func (lb *LoadBalancerRR) resetService(svcPort iptables.ServicePortName) {
	// If the service is still around, reset but don't delete.
	if state, ok := lb.services[svcPort]; ok {
		if len(state.endpoints) > 0 {
			klog.V(2).Infof("LoadBalancerRR: Removing endpoints for %s", svcPort)
			state.endpoints = []string{}
		}
		state.index = 0
		state.affinity.affinityMap = map[string]*affinityState{}
	}
}

func (lb *LoadBalancerRR) OnEndpointsDelete(ep *localnetv1.Endpoint, svc *localnetv1.Service) {
	portsToEndpoints := buildPortsToEndpointsMap(ep, svc)

	lb.lock.Lock()
	defer lb.lock.Unlock()

	for portname := range portsToEndpoints {
		svcPort := iptables.ServicePortName{NamespacedName: types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, Port: portname}
		lb.resetService(svcPort)
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
	if reflect.DeepEqual(lhs, rhs) {
		return true
	}
	return false
}

func (lb *LoadBalancerRR) CleanupStaleStickySessions(svcPort iptables.ServicePortName) {
	lb.lock.Lock()
	defer lb.lock.Unlock()

	state, exists := lb.services[svcPort]
	if !exists {
		return
	}
	for ip, affinity := range state.affinity.affinityMap {
		if int(time.Since(affinity.lastUsed).Seconds()) >= state.affinity.ttlSeconds {
			klog.V(4).Infof("Removing client %s from affinityMap for service %q", affinity.clientIP, svcPort)
			delete(state.affinity.affinityMap, ip)
		}
	}
}
