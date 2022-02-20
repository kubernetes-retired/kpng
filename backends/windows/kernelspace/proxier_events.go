package kernelspace

import (
	"sync/atomic"

	discovery "k8s.io/api/discovery/v1"
	netutils "k8s.io/utils/net"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/api/localnetv1"
)

// OnEndpointsAdd is called whenever creation of new windowsEndpoint object
// is observed.
func (Proxier *Proxier) OnEndpointsAdd(ep *localnetv1.Endpoint, svc *localnetv1.Service) {}

// OnEndpointsUpdate is called whenever modification of an existing
// windowsEndpoint object is observed.
func (proxier *Proxier) OnEndpointsUpdate(oldEndpoints, endpoints *localnetv1.Endpoint) {}

// OnEndpointsDelete is called whenever deletion of an existing windowsEndpoint
// object is observed. Service object
func (Proxier *Proxier) OnEndpointsDelete(ep *localnetv1.Endpoint, svc *localnetv1.Service) {}

// OnEndpointsSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (Proxier *Proxier) OnEndpointsSynced() {}

// TODO Fix EndpointSlices logic !!!!!!!!!!!!! JAY
func (Proxier *Proxier) OnEndpointSliceAdd(endpointSlice *discovery.EndpointSlice) {
	//	if Proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && Proxier.isInitialized() {
	//		Proxier.Sync()
	//	}
}
func (Proxier *Proxier) OnEndpointSliceUpdate(_, endpointSlice *discovery.EndpointSlice) {
	//	if Proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && Proxier.isInitialized() {
	//		Proxier.Sync()
	//	}
}
func (Proxier *Proxier) OnEndpointSliceDelete(endpointSlice *discovery.EndpointSlice) {
	//	if Proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, true) && Proxier.isInitialized() {
	//		proxier.Sync()
	//	}
}

func (Proxier *Proxier) BackendDeleteService(
	namespace string,
	name string) {

	svcPortName := ServicePortName{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name}}

	_, exists := Proxier.serviceMap[svcPortName]
	if exists {
		Proxier.serviceMap[svcPortName] = nil
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

func (Proxier *Proxier) endpointsMapChange(oldEndpointsMap, newEndpointsMap EndpointsMap) {
	//read the old windowsEndpoint...

	// iterate through this cache.. map[types.NamespacedName]*endpointsInfoByName
	for svcPortName, _ := range oldEndpointsMap {
		spn := &ServicePortName{
			NamespacedName: svcPortName,
			// Port:
			// Protocol:
		}
		Proxier.onEndpointsMapChange(spn)
	}

	//read the new windowsEndpoint...
	for svcPortName := range newEndpointsMap {
		spn := &ServicePortName{
			NamespacedName: svcPortName,
			// Port:
			// Protocol:
		}
		Proxier.onEndpointsMapChange(spn)
	}
}

func (Proxier *Proxier) onEndpointsMapChange(svcPortName *ServicePortName) {

	svc, exists := Proxier.serviceMap[*svcPortName]

	if exists {
		svcInfo, ok := svc.(*serviceInfo)

		if !ok {
			klog.ErrorS(nil, "Failed to cast serviceInfo", "servicePortName", svcPortName)
			return
		}

		klog.V(3).InfoS("Endpoints are modified. Service is stale", "servicePortName", svcPortName)
		spn := &ServicePortName{
			NamespacedName: svcPortName.NamespacedName,
			// Port:
			// Protocol:
		}

		// e := Proxier.endpointsMap[spn.NamespacedName]
		endpoints := Proxier.endpointsMap[spn.NamespacedName]
		for _, e := range *endpoints {
			svcInfo.cleanupAllPolicies(e)
		}
	} else {
		// If no service exists, just cleanup the remote windowsEndpoint
		klog.V(3).InfoS("Endpoints are orphaned, cleaning up")
		// Cleanup Endpoints references

		// TODO: Jay fix endpoint cleanup logic : what should happen here? look back in original windows
		//epInfos, exists := Proxier.endpointsMap[svcPortName.NamespacedName]
		// proxy .
		// if exists {
		// Cleanup Endpoints references
		//	for _, ep := range *epInfos {
		//		ep.
		//		}
		//}
	}
}

func (Proxier *Proxier) serviceMapChange(previous, current ServiceMap) {
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

func (Proxier *Proxier) onServiceMapChange(svcPortName *ServicePortName) {

	// the ServicePort interface is used to store serviceInfo objects...
	spn := &ServicePortName{
		NamespacedName: svcPortName.NamespacedName,
		// Port:
		// Protocol:
	}
	svc, exists := Proxier.serviceMap[*spn]

	if exists {
		// The generic ServicePort interface casts down to a specific windows implementation here... "serviceInfo"...
		svcInfo, ok := svc.(*serviceInfo)

		if !ok {
			klog.ErrorS(nil, "Failed to cast serviceInfo", "servicePortName", svcPortName)
			return
		}

		klog.V(3).InfoS(
			"Updating existing service port",
			"servicePortName", svcPortName,
			"clusterIP", svcInfo.ClusterIP(),
			"port", svcInfo.Port(),
			"protocol", svcInfo.Protocol(),
		)
		endpoints := Proxier.endpointsMap[spn.NamespacedName]
		for _, e := range *endpoints {
			svcInfo.cleanupAllPolicies(e)
		}

	}
}

// returns a new proxy.Endpoint which abstracts a endpointsInfo
func (Proxier *Proxier) newEndpointInfo(baseInfo *BaseEndpointInfo) Endpoint {
	portNumber, err := baseInfo.Port()

	if err != nil {
		portNumber = 0
	}

	info := &windowsEndpoint{
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
func (Proxier *Proxier) newServiceInfo(port *localnetv1.PortMapping, service *localnetv1.Service, baseInfo *BaseServiceInfo) ServicePort {
	info := &serviceInfo{BaseServiceInfo: baseInfo}
	preserveDIP := service.Annotations["preserve-destination"] == "true"

	// TODO Jay , figure out how to implement DSR at some point...
	//	localTrafficDSR := service.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal
	// err := hcn.DSRSupported()
	//if err != nil {
	// preserveDIP := false
	localTrafficDSR := false
	//}

	// TODO Jay , jsut making this compile ignorignt the intstr int parser not needed i think
	// targetPort is zero if it is specified as a name in port.TargetPort.
	// Its real value would be got later from windowsEndpoint.
	targetPort := 0
	//if port.TargetPort == intstr.Int {
	targetPort = int(port.TargetPort)
	//}

	info.preserveDIP = preserveDIP
	info.targetPort = targetPort
	info.hns = Proxier.hns
	info.localTrafficDSR = localTrafficDSR

	// TODO Jay: Adding both v4 and v6 ips.  no idea if this breaks dualstack or not.
	for _, eip := range service.IPs.ExternalIPs.V4 {
		info.externalIPs = append(info.externalIPs, &externalIPInfo{ip: eip})
	}
	for _, eip := range service.IPs.ExternalIPs.V6 {
		info.externalIPs = append(info.externalIPs, &externalIPInfo{ip: eip})
	}

	// TODO: Jay: What should we do for loadbalancer ingress IPs? Are they any different in kpng or just reuse v4/v6
	/**
	for _, ingress := range service.LoadBalancer.Ingress {
		if netutils.ParseIPSloppy(ingress.IP) != nil {
			info.loadBalancerIngressIPs = append(info.loadBalancerIngressIPs, &loadBalancerIngressInfo{ip: ingress.IP})
		}
	}
	*/
	return info
}

func (Proxier *Proxier) setInitialized(value bool) {
	var initialized int32
	if value {
		initialized = 1
	}
	atomic.StoreInt32(&Proxier.initialized, initialized)
}
