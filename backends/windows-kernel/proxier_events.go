package winkernel

import (
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	netutils "k8s.io/utils/net"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
        "k8s.io/kubernetes/pkg/proxy"
	"github.com/Microsoft/hcsshim/hcn"
)

// OnEndpointsAdd is called whenever creation of new endpoints object
// is observed.
func (proxier *Proxier) OnEndpointsAdd(endpoints *v1.Endpoints) {}

// OnEndpointsDelete is called whenever deletion of an existing endpoints
// object is observed.
func (proxier *Proxier) OnEndpointsDelete(endpoints *v1.Endpoints) {}

// OnServiceAdd is called whenever creation of new service object
// is observed.
func (proxier *Proxier) OnServiceAdd(service *v1.Service) {
        proxier.OnServiceUpdate(nil, service)
}

// OnEndpointsUpdate is called whenever modification of an existing
// endpoints object is observed.
func (proxier *Proxier) OnEndpointsUpdate(oldEndpoints, endpoints *v1.Endpoints) {}


// OnEndpointsSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnEndpointsSynced() {}

// OnServiceUpdate is called whenever modification of an existing
// service object is observed.
func (proxier *Proxier) OnServiceUpdate(oldService, service *v1.Service) {
        if proxier.serviceChanges.Update(oldService, service) && proxier.isInitialized() {
                proxier.Sync()
        }
}

// OnServiceDelete is called whenever deletion of an existing service
// object is observed.
func (proxier *Proxier) OnServiceDelete(service *v1.Service) {
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

// OnEndpointSliceAdd is called whenever creation of a new endpoint slice object
// is observed.
func (proxier *Proxier) OnEndpointSliceAdd(endpointSlice *discovery.EndpointSlice) {
        if proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && proxier.isInitialized() {
                proxier.Sync()
        }
}

// OnEndpointSliceUpdate is called whenever modification of an existing endpoint
// slice object is observed.
func (proxier *Proxier) OnEndpointSliceUpdate(_, endpointSlice *discovery.EndpointSlice) {
        if proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, false) && proxier.isInitialized() {
                proxier.Sync()
        }
}

// OnEndpointSliceDelete is called whenever deletion of an existing endpoint slice
// object is observed.
func (proxier *Proxier) OnEndpointSliceDelete(endpointSlice *discovery.EndpointSlice) {
        if proxier.endpointsChanges.EndpointSliceUpdate(endpointSlice, true) && proxier.isInitialized() {
                proxier.Sync()
        }
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

func (proxier *Proxier) endpointsMapChange(oldEndpointsMap, newEndpointsMap proxy.EndpointsMap) {
        for svcPortName := range oldEndpointsMap {
                proxier.onEndpointsMapChange(&svcPortName)
        }

        for svcPortName := range newEndpointsMap {
                proxier.onEndpointsMapChange(&svcPortName)
        }
}

func (proxier *Proxier) onEndpointsMapChange(svcPortName *proxy.ServicePortName) {

        svc, exists := proxier.serviceMap[*svcPortName]

        if exists {
                svcInfo, ok := svc.(*serviceInfo)

                if !ok {
                        klog.ErrorS(nil, "Failed to cast serviceInfo", "servicePortName", svcPortName)
                        return
                }

                klog.V(3).InfoS("Endpoints are modified. Service is stale", "servicePortName", svcPortName)
                svcInfo.cleanupAllPolicies(proxier.endpointsMap[*svcPortName])
        } else {
                // If no service exists, just cleanup the remote endpoints
                klog.V(3).InfoS("Endpoints are orphaned, cleaning up")
                // Cleanup Endpoints references
                epInfos, exists := proxier.endpointsMap[*svcPortName]

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

func (proxier *Proxier) serviceMapChange(previous, current proxy.ServiceMap) {
        for svcPortName := range current {
                proxier.onServiceMapChange(&svcPortName)
        }

        for svcPortName := range previous {
                if _, ok := current[svcPortName]; ok {
                        continue
                }
                proxier.onServiceMapChange(&svcPortName)
        }
}

func (proxier *Proxier) onServiceMapChange(svcPortName *proxy.ServicePortName) {

        svc, exists := proxier.serviceMap[*svcPortName]

        if exists {
                svcInfo, ok := svc.(*serviceInfo)

                if !ok {
                        klog.ErrorS(nil, "Failed to cast serviceInfo", "servicePortName", svcPortName)
                        return
                }

                klog.V(3).InfoS("Updating existing service port", "servicePortName", svcPortName, "clusterIP", svcInfo.ClusterIP(), "port", svcInfo.Port(), "protocol", svcInfo.Protocol())
                svcInfo.cleanupAllPolicies(proxier.endpointsMap[*svcPortName])
        }
}

// returns a new proxy.Endpoint which abstracts a endpointsInfo
func (proxier *Proxier) newEndpointInfo(baseInfo *proxy.BaseEndpointInfo) proxy.Endpoint {
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
                hns:        proxier.hns,

                ready:       baseInfo.Ready,
                serving:     baseInfo.Serving,
                terminating: baseInfo.Terminating,
        }

        return info
}

// returns a new proxy.ServicePort which abstracts a serviceInfo
func (proxier *Proxier) newServiceInfo(port *v1.ServicePort, service *v1.Service, baseInfo *proxy.BaseServiceInfo) proxy.ServicePort {
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
        info.hns = proxier.hns
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


