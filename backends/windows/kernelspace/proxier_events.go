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
	"sync/atomic"

	discovery "k8s.io/api/discovery/v1"
	netutils "k8s.io/utils/net"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kpng/api/localv1"
)

// OnEndpointsAdd is called whenever creation of new windowsEndpoint object
// is observed.
//func (proxier *Proxier) OnEndpointsAdd(ep *kpng.Endpoint, svc *kpng.Service) {
//	baseInfo := &BaseEndpointInfo{
//		Endpoint:    "TODO what is this supposed to be?",
//		IsLocal:     ep.Local,
//		ZoneHints:   map[string]sets.Empty{"TODO what is this?": {}},
//		Ready:       false, // TODO
//		Serving:     false, // TODO
//		Terminating: false, // TODO
//		NodeName:    ep.Hostname,
//		Zone:        "TODO what is this?",
//	}
//	we := proxier.newWindowsEndpointFromBaseEndpointInfo(baseInfo)
//	proxier.kpngEndpointCache.storeEndpoint(*ep, we)
//}

// OnEndpointsUpdate is called whenever modification of an existing
// windowsEndpoint object is observed.
//func (proxier *Proxier) OnEndpointsUpdate(oldEndpoints, endpoints *kpng.Endpoint) {
//	proxier.kpngEndpointCache.removeEndpoint(oldEndpoints)
//
//	baseInfo := &BaseEndpointInfo{
//		Endpoint:    "TODO what is this supposed to be?",
//		IsLocal:     endpoints.Local,
//		ZoneHints:   map[string]sets.Empty{"TODO what is this?": {}},
//		Ready:       false, // TODO
//		Serving:     false, // TODO
//		Terminating: false, // TODO
//		NodeName:    endpoints.Hostname,
//		Zone:        "TODO what is this?",
//	}
//	we := proxier.newWindowsEndpointFromBaseEndpointInfo(baseInfo)
//	proxier.kpngEndpointCache.storeEndpoint(*endpoints, we)
//}

// OnEndpointsDelete is called whenever deletion of an existing windowsEndpoint
// object is observed. Service object
//func (proxier *Proxier) OnEndpointsDelete(ep *kpng.Endpoint, svc *kpng.Service) {
//	proxier.kpngEndpointCache.removeEndpoint(ep)
//}

// OnEndpointsSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnEndpointsSynced() {
	// TODO
}

// TODO Fix EndpointSlices logic !!!!!!!!!!!!! JAY
func (proxier *Proxier) OnEndpointSliceAdd(endpointSlice *discovery.EndpointSlice) {
	//	if Proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && Proxier.isInitialized() {
	//		Proxier.Sync()
	//	}
}
func (proxier *Proxier) OnEndpointSliceUpdate(_, endpointSlice *discovery.EndpointSlice) {
	//	if Proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && Proxier.isInitialized() {
	//		Proxier.Sync()
	//	}
}
func (proxier *Proxier) OnEndpointSliceDelete(endpointSlice *discovery.EndpointSlice) {
	//	if Proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, true) && Proxier.isInitialized() {
	//		proxier.Sync()
	//	}
}

func (proxier *Proxier) BackendDeleteService(namespace string, name string) {
	a := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	delete(proxier.serviceMap, a)
}

// OnEndpointSlicesSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnEndpointSlicesSynced() {
	proxier.mu.Lock()
	proxier.endpointSlicesSynced = true
	proxier.setInitialized(proxier.servicesSynced)
	proxier.mu.Unlock()

	// Sync unconditionally - this is called once per lifetime.
	proxier.syncProxyRules()
}

// OnServiceAdd is called whenever creation of new service object
// is observed.
func (proxier *Proxier) OnServiceAdd(service *localv1.Service) {
	proxier.OnServiceUpdate(nil, service)
}

// OnServiceUpdate is called whenever modification of an existing
// service object is observed.
func (proxier *Proxier) OnServiceUpdate(oldService, service *localv1.Service) {
	proxier.Sync()
}

// OnServiceDelete is called whenever deletion of an existing service
// object is observed.
func (proxier *Proxier) OnServiceDelete(service *localv1.Service) {
	proxier.OnServiceUpdate(service, nil)
}

// OnServiceSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnServiceSynced() {
	proxier.mu.Lock()
	proxier.servicesSynced = true
	proxier.setInitialized(proxier.endpointSlicesSynced)
	proxier.mu.Unlock()

	// Sync unconditionally - this is called once per lifetime.
	proxier.syncProxyRules()
}

func (proxier *Proxier) newEndpointInfo(baseInfo *BaseEndpointInfo, _ *ServicePortName) *endpointsInfo {

	portNumber, err := baseInfo.Port()

	if err != nil {
		portNumber = 0
	}

	info := &endpointsInfo{
		ip:         baseInfo.IP(),
		port:       uint16(portNumber),
		isLocal:    baseInfo.GetIsLocal(),
		macAddress: conjureMac("02-11", netutils.ParseIPSloppy(baseInfo.IP())),
		refCount:   new(uint16),
		hnsID:      "",
		hns:        proxier.hns,

		ready:       baseInfo.Ready,
		serving:     baseInfo.Serving,
		terminating: baseInfo.Terminating,
	}

	return info
}

func (proxier *Proxier) newServiceInfo(port *localv1.PortMapping, service *localv1.Service, baseInfo *BaseServiceInfo) ServicePort {
	info := &serviceInfo{BaseServiceInfo: baseInfo}
	preserveDIP := service.Annotations["preserve-destination"] == "true"
	//localTrafficDSR := service.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal
	//err := hcn.DSRSupported()
	//if err != nil {
	//    preserveDIP = false
	//    localTrafficDSR := false
	//}
	localTrafficDSR := false
	// targetPort is zero if it is specified as a name in port.TargetPort.
	// Its real value would be got later from endpoints.
	targetPort := 0
	//if port.TargetPort.Type == intstr.Int {
	targetPort = int(port.TargetPort)
	//}

	info.preserveDIP = preserveDIP
	info.targetPort = targetPort
	info.hns = proxier.hns
	info.localTrafficDSR = localTrafficDSR

	for _, eip := range service.IPs.ExternalIPs.V4 {
		info.externalIPs = append(info.externalIPs, &externalIPInfo{ip: eip})
	}
	for _, eip := range service.IPs.ExternalIPs.V6 {
		info.externalIPs = append(info.externalIPs, &externalIPInfo{ip: eip})
	}

	//for _, ingress := range service.Status.LoadBalancer.Ingress {
	//    if netutils.ParseIPSloppy(ingress.IP) != nil {
	//        info.loadBalancerIngressIPs = append(info.loadBalancerIngressIPs, &loadBalancerIngressInfo{ip: ingress.IP})
	//    }
	//}
	return info
}

func (proxier *Proxier) setInitialized(value bool) {
	var initialized int32
	if value {
		initialized = 1
	}
	atomic.StoreInt32(&proxier.initialized, initialized)
}
