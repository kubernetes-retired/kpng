package kernelspace

import (
	"sync/atomic"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	netutils "k8s.io/utils/net"

	"github.com/Microsoft/hcsshim/hcn"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/proxy"
	"sigs.k8s.io/kpng/api/localnetv1"
)

// OnEndpointsAdd is called whenever creation of new endpoints object
// is observed.
func (Proxier *Proxier) OnEndpointsAdd(ep *localnetv1.Endpoint, svc *localnetv1.Service) {}

// OnEndpointsUpdate is called whenever modification of an existing
// endpoints object is observed.
func (proxier *Proxier) OnEndpointsUpdate(oldEndpoints, endpoints *localnetv1.Endpoint) {}

// OnEndpointsDelete is called whenever deletion of an existing endpoints
// object is observed. Service object
func (Proxier *Proxier) OnEndpointsDelete(ep *localnetv1.Endpoint, svc *localnetv1.Service) {}

// OnEndpointsSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (Proxier *Proxier) OnEndpointsSynced() {}

// OnEndpointSliceAdd is called whenever creation of a new endpoint slice object
// is observed.
func (Proxier *Proxier) OnEndpointSliceAdd(endpointSlice *discovery.EndpointSlice) {
	if Proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && Proxier.isInitialized() {
		Proxier.Sync()
	}
}

// OnEndpointSliceUpdate is called whenever modification of an existing endpoint
// slice object is observed.
func (Proxier *Proxier) OnEndpointSliceUpdate(_, endpointSlice *discovery.EndpointSlice) {
	if Proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && Proxier.isInitialized() {
		Proxier.Sync()
	}
}

func (Proxier *Proxier) BackendDeleteService(
	namespace string,
	name string) {

	svcPortName := proxy.ServicePortName{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name}}

	_, exists := Proxier.serviceMap[svcPortName]
	if exists {
		Proxier.serviceMap[svcPortName] = nil
	}
}

// OnEndpointSliceDelete is called whenever deletion of an existing endpoint slice
// object is observed.
func (Proxier *Proxier) OnEndpointSliceDelete(endpointSlice *discovery.EndpointSlice) {
	if Proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, true) && Proxier.isInitialized() {
		proxier.Sync()
	}
}

// OnEndpointSlicesSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (Proxier *Proxier) OnEndpointSlicesSynced() {
	Proxier.mu.Lock()
	Proxier.endpointSlicesSynced = true
	Proxier.setInitialized(Proxier.servicesSynced)
	Proxier.mu.Unlock()

	// Sync unconditionally - this is called once per lifetime.
	Proxier.syncProxyRules()
}

// OnServiceAdd is called whenever creation of new service object
// is observed.
func (proxier *Proxier) OnServiceAdd(service *localnetv1.Service) {
	proxier.OnServiceUpdate(nil, service)
}

// OnServiceUpdate is called whenever modification of an existing
// service object is observed.
func (proxier *Proxier) OnServiceUpdate(oldService, service *localnetv1.Service) {
	proxier.Sync()
}

// OnServiceDelete is called whenever deletion of an existing service
// object is observed.
func (proxier *Proxier) OnServiceDelete(service *localnetv1.Service) {
	proxier.OnServiceUpdate(service, nil)
}

// OnServiceSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (Proxier *Proxier) OnServiceSynced() {
	Proxier.mu.Lock()
	Proxier.servicesSynced = true
	Proxier.setInitialized(Proxier.endpointSlicesSynced)
	Proxier.mu.Unlock()

	// Sync unconditionally - this is called once per lifetime.
	Proxier.syncProxyRules()
}

func (Proxier *Proxier) endpointsMapChange(oldEndpointsMap, newEndpointsMap proxy.EndpointsMap) {
	for svcPortName := range oldEndpointsMap {
		Proxier.onEndpointsMapChange(&svcPortName)
	}

	for svcPortName := range newEndpointsMap {
		Proxier.onEndpointsMapChange(&svcPortName)
	}
}

func (Proxier *Proxier) onEndpointsMapChange(svcPortName *proxy.ServicePortName) {

	svc, exists := Proxier.serviceMap[*svcPortName]

	if exists {
		svcInfo, ok := svc.(*serviceInfo)

		if !ok {
			klog.ErrorS(nil, "Failed to cast serviceInfo", "servicePortName", svcPortName)
			return
		}

		klog.V(3).InfoS("Endpoints are modified. Service is stale", "servicePortName", svcPortName)
		svcInfo.cleanupAllPolicies(Proxier.endpointsMap[*svcPortName])
	} else {
		// If no service exists, just cleanup the remote endpoints
		klog.V(3).InfoS("Endpoints are orphaned, cleaning up")
		// Cleanup Endpoints references
		epInfos, exists := Proxier.endpointsMap[*svcPortName]

		if exists {
			// Cleanup Endpoints references
			for _, ep := range epInfos {
				epInfo, ok := ep.(*endpoints)

				if ok {
					epInfo.Cleanup()
				}

			}
		}
	}
}

func (Proxier *Proxier) serviceMapChange(previous, current proxy.ServiceMap) {
	for svcPortName := range current {
		Proxier.onServiceMapChange(&svcPortName)
	}

	for svcPortName := range previous {
		if _, ok := current[svcPortName]; ok {
			continue
		}
		Proxier.onServiceMapChange(&svcPortName)
	}
}

func (Proxier *Proxier) onServiceMapChange(svcPortName *proxy.ServicePortName) {

	svc, exists := Proxier.serviceMap[*svcPortName]

	if exists {
		svcInfo, ok := svc.(*serviceInfo)

		if !ok {
			klog.ErrorS(nil, "Failed to cast serviceInfo", "servicePortName", svcPortName)
			return
		}

		klog.V(3).InfoS("Updating existing service port", "servicePortName", svcPortName, "clusterIP", svcInfo.ClusterIP(), "port", svcInfo.Port(), "protocol", svcInfo.Protocol())
		svcInfo.cleanupAllPolicies(Proxier.endpointsMap[*svcPortName])
	}
}

// returns a new proxy.Endpoint which abstracts a endpointsInfo
func (Proxier *Proxier) newEndpointInfo(baseInfo *proxy.BaseEndpointInfo) proxy.Endpoint {
	portNumber, err := baseInfo.Port()

	if err != nil {
		portNumber = 0
	}

	info := &endpoints{
		ip:         baseInfo.IP(),
		port:       uint16(portNumber),
		isLocal:    baseInfo.GetIsLocal(),
		macAddress: conjureMac("02-11", netutils.ParseIPSloppy(baseInfo.IP())),
		refCount:   new(uint16),
		hnsID:      "",
		hns:        Proxier.hns,

		ready:       baseInfo.Ready,
		serving:     baseInfo.Serving,
		terminating: baseInfo.Terminating,
	}

	return info
}

// returns a new proxy.ServicePort which abstracts a serviceInfo
func (Proxier *Proxier) newServiceInfo(port *v1.ServicePort, service *v1.Service, baseInfo *proxy.BaseServiceInfo) proxy.ServicePort {
	info := &serviceInfo{BaseServiceInfo: baseInfo}
	preserveDIP := service.Annotations["preserve-destination"] == "true"
	localTrafficDSR := service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal
	err := hcn.DSRSupported()
	if err != nil {
		preserveDIP = false
		localTrafficDSR = false
	}
	// targetPort is zero if it is specified as a name in port.TargetPort.
	// Its real value would be got later from endpoints.
	targetPort := 0
	if port.TargetPort.Type == intstr.Int {
		targetPort = port.TargetPort.IntValue()
	}

	info.preserveDIP = preserveDIP
	info.targetPort = targetPort
	info.hns = Proxier.hns
	info.localTrafficDSR = localTrafficDSR

	for _, eip := range service.Spec.ExternalIPs {
		info.externalIPs = append(info.externalIPs, &externalIPInfo{ip: eip})
	}

	for _, ingress := range service.Status.LoadBalancer.Ingress {
		if netutils.ParseIPSloppy(ingress.IP) != nil {
			info.loadBalancerIngressIPs = append(info.loadBalancerIngressIPs, &loadBalancerIngressInfo{ip: ingress.IP})
		}
	}
	return info
}

func (Proxier *Proxier) setInitialized(value bool) {
	var initialized int32
	if value {
		initialized = 1
	}
	atomic.StoreInt32(&Proxier.initialized, initialized)
}
