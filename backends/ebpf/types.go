/*
Copyright 2015 The Kubernetes Authors.

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

package ebpf

import (
	"fmt"
	"net"
	"sync"

	cebpflink "github.com/cilium/ebpf/link"
	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Userspace Types
type svcEndpointMapping struct {
	Svc *BaseServiceInfo

	Endpoint []*localnetv1.Endpoint
}

type ebpfController struct {
	// protects the following fields
	mu sync.Mutex

	// Keeps track of ebpf objects in memory.
	objs bpfObjects

	// Program Link,
	bpfLink cebpflink.Link

	ipFamily v1.IPFamily

	// Caches of what service info our ebpf MAPs should contain
	svcMap map[string]svcEndpointMapping

	// Cache of all service keys in the svcMap, used to build deletion lists
	svcMapKeys sets.String
}

func NewEBPFController(objs bpfObjects, bpfProgLink cebpflink.Link, ipFamily v1.IPFamily) ebpfController {
	return ebpfController{
		objs:       objs,
		bpfLink:    bpfProgLink,
		ipFamily:   ipFamily,
		svcMap:     make(map[string]svcEndpointMapping),
		svcMapKeys: sets.NewString(),
	}
}

// ServicePortName carries a namespace + name + portname.  This is the unique
// identifier for a load-balanced service.
type ServicePortName struct {
	types.NamespacedName
	Port     string
	Protocol localnetv1.Protocol
}

func (spn ServicePortName) String() string {
	return fmt.Sprintf("%s%s%s", spn.NamespacedName.String(), fmtPortName(spn.Port), spn.Protocol)
}

func fmtPortName(in string) string {
	if in == "" {
		return ""
	}
	return fmt.Sprintf(":%s", in)
}

// ServicePort is an interface which abstracts information about a service.
type ServicePort interface {
	// String returns service string.  An example format can be: `IP:Port/Protocol`.
	String() string
	// GetClusterIP returns service cluster IP in net.IP format.
	ClusterIP() net.IP
	// GetPort returns service port if present. If return 0 means not present.
	Port() int

	// TODO not yet implemented for ebpf backends
	// GetSessionAffinityType returns service session affinity type
	// SessionAffinity() SessionAffinity

	// ExternalIPStrings returns service ExternalIPs as a string array.
	ExternalIPStrings() []string
	// LoadBalancerIPStrings returns service LoadBalancerIPs as a string array.
	LoadBalancerIPStrings() []string
	// GetProtocol returns service protocol.
	Protocol() localnetv1.Protocol
	// LoadBalancerSourceRanges returns service LoadBalancerSourceRanges if present empty array if not
	LoadBalancerSourceRanges() []string
	// GetHealthCheckNodePort returns service health check node port if present.  If return 0, it means not present.
	HealthCheckNodePort() int
	// GetNodePort returns a service Node port if present. If return 0, it means not present.
	NodePort() int
	// NodeLocalExternal returns if a service has only node local endpoints for external traffic.
	NodeLocalExternal() bool
	// NodeLocalInternal returns if a service has only node local endpoints for internal traffic.
	NodeLocalInternal() bool
	// InternalTrafficPolicy returns service InternalTrafficPolicy
	InternalTrafficPolicy() *v1.ServiceInternalTrafficPolicyType
	// HintsAnnotation returns the value of the v1.AnnotationTopologyAwareHints annotation.
	HintsAnnotation() string
}

// BaseServiceInfo contains base information that defines a service.
// This could be used directly by proxier while processing services,
// or can be used for constructing a more specific ServiceInfo struct
// defined by the proxier if needed.
type BaseServiceInfo struct {
	clusterIP                net.IP
	port                     int
	protocol                 localnetv1.Protocol
	nodePort                 int
	loadBalancerIPs          []string
	sessionAffinity          SessionAffinity
	stickyMaxAgeSeconds      int
	externalIPs              []string
	loadBalancerSourceRanges []string
	healthCheckNodePort      int
	nodeLocalExternal        bool
	nodeLocalInternal        bool
	internalTrafficPolicy    *v1.ServiceInternalTrafficPolicyType
	hintsAnnotation          string
	targetPort               int
	targetPortName           string
	portName                 string
}

// SessionAffinity contains data about assinged session affinity
type SessionAffinity struct {
	ClientIP *localnetv1.Service_ClientIP
}

var _ ServicePort = &BaseServiceInfo{}

// String is part of ServicePort interface.
func (info *BaseServiceInfo) String() string {
	return fmt.Sprintf("%s:%d/%s", info.clusterIP, info.port, info.protocol)
}

// ClusterIP is part of ServicePort interface.
func (info *BaseServiceInfo) ClusterIP() net.IP {
	return info.clusterIP
}

// Port is part of ServicePort interface.
func (info *BaseServiceInfo) Port() int {
	return info.port
}

// Port is part of ServicePort interface.
func (info *BaseServiceInfo) TargetPort() int {
	return info.targetPort
}

// PortName is part of ServicePort interface.
func (info *BaseServiceInfo) PortName() string {
	return info.portName
}

func (info *BaseServiceInfo) TargetPortName() string {
	return info.targetPortName
}

// SessionAffinity is part of the ServicePort interface.
func (info *BaseServiceInfo) SessionAffinity() SessionAffinity {
	return info.sessionAffinity
}

// Protocol is part of ServicePort interface.
func (info *BaseServiceInfo) Protocol() localnetv1.Protocol {
	return info.protocol
}

// LoadBalancerSourceRanges is part of ServicePort interface
func (info *BaseServiceInfo) LoadBalancerSourceRanges() []string {
	return info.loadBalancerSourceRanges
}

// HealthCheckNodePort is part of ServicePort interface.
func (info *BaseServiceInfo) HealthCheckNodePort() int {
	return info.healthCheckNodePort
}

// NodePort is part of the ServicePort interface.
func (info *BaseServiceInfo) NodePort() int {
	return info.nodePort
}

// ExternalIPStrings is part of ServicePort interface.
func (info *BaseServiceInfo) ExternalIPStrings() []string {
	return info.externalIPs
}

// LoadBalancerIPStrings is part of ServicePort interface.
func (info *BaseServiceInfo) LoadBalancerIPStrings() []string {
	var ips []string
	for _, ing := range info.loadBalancerIPs {
		ips = append(ips, ing)
	}
	return ips
}

// NodeLocalExternal is part of ServicePort interface.
func (info *BaseServiceInfo) NodeLocalExternal() bool {
	return info.nodeLocalExternal
}

// NodeLocalInternal is part of ServicePort interface
func (info *BaseServiceInfo) NodeLocalInternal() bool {
	return info.nodeLocalInternal
}

// InternalTrafficPolicy is part of ServicePort interface
func (info *BaseServiceInfo) InternalTrafficPolicy() *v1.ServiceInternalTrafficPolicyType {
	return info.internalTrafficPolicy
}

// HintsAnnotation is part of ServicePort interface.
func (info *BaseServiceInfo) HintsAnnotation() string {
	return info.hintsAnnotation
}

func (sct *ebpfController) newBaseServiceInfo(port *localnetv1.PortMapping, service *localnetv1.Service) *BaseServiceInfo {
	nodeLocalExternal := false
	if RequestsOnlyLocalTraffic(service) {
		nodeLocalExternal = true
	}
	nodeLocalInternal := false
	// TODO : CHECK InternalTrafficPolicy
	// if utilfeature.DefaultFeatureGate.Enabled(features.ServiceInternalTrafficPolicy) {
	// 	nodeLocalInternal = apiservice.RequestsOnlyLocalTrafficForInternal(service)
	// }

	clusterIP := GetClusterIPByFamily(sct.ipFamily, service)
	info := &BaseServiceInfo{
		clusterIP:         net.ParseIP(clusterIP),
		port:              int(port.Port),
		portName:          port.Name,
		targetPort:        int(port.TargetPort),
		targetPortName:    port.TargetPortName,
		protocol:          port.Protocol,
		nodePort:          int(port.NodePort),
		nodeLocalExternal: nodeLocalExternal,
		nodeLocalInternal: nodeLocalInternal,
		// internalTrafficPolicy: service.Spec.InternalTrafficPolicy, //TODO : CHECK InternalTrafficPolicy
		hintsAnnotation:          service.Annotations[v1.AnnotationTopologyAwareHints],
		loadBalancerSourceRanges: getLoadbalancerSourceRanges(service.IPFilters),
		loadBalancerIPs:          getLoadBalancerIPs(service.IPs.LoadBalancerIPs, sct.ipFamily),
		sessionAffinity:          getSessionAffinity(service.SessionAffinity),
	}

	// filter external ips, source ranges and ingress ips
	// prior to dual stack services, this was considered an error, but with dual stack
	// services, this is actually expected. Hence we downgraded from reporting by events
	// to just log lines with high verbosity

	//ipFamilyMap := MapIPsByIPFamily(service.IPs.ExternalIPs)
	//info.externalIPs = ipFamilyMap[sct.ipFamily]

	// Log the IPs not matching the ipFamily
	//if ips, ok := ipFamilyMap[OtherIPFamily(sct.ipFamily)]; ok && len(ips) > 0 {
	//	klog.V(4).Infof("service change tracker(%v) ignored the following external IPs(%s) for service %v/%v as they don't match IPFamily", sct.ipFamily, strings.Join(ips, ","), service.Namespace, service.Name)
	//}

	//TODO : CHECK service.Spec.HealthCheckNodePort
	// if apiservice.NeedsHealthCheck(service) {
	// 	p := service.Spec.HealthCheckNodePort
	// 	if p == 0 {
	// 		klog.Errorf("Service %s/%s has no healthcheck nodeport", service.Namespace, service.Name)
	// 	} else {
	// 		info.healthCheckNodePort = int(p)
	// 	}
	// }

	return info
}

// GetClusterIPByFamily returns a service clusterip by family
func GetClusterIPByFamily(ipFamily v1.IPFamily, service *localnetv1.Service) string {
	if ipFamily == v1.IPv4Protocol {
		if len(service.IPs.ClusterIPs.V4) > 0 {
			return service.IPs.ClusterIPs.V4[0]
		}
	}
	if ipFamily == v1.IPv6Protocol {
		if len(service.IPs.ClusterIPs.V6) > 0 {
			return service.IPs.ClusterIPs.V6[0]
		}
	}
	return ""
}

func getSessionAffinity(affinity interface{}) SessionAffinity {
	var sessionAffinity SessionAffinity
	switch affinity.(type) {
	case *localnetv1.Service_ClientIP:
		sessionAffinity.ClientIP = affinity.(*localnetv1.Service_ClientIP)
	}
	return sessionAffinity
}

func getLoadBalancerIPs(ips *localnetv1.IPSet, ipFamily v1.IPFamily) []string {
	if ips == nil {
		return nil
	}
	if ipFamily == v1.IPv4Protocol {
		return ips.V4
	}
	return ips.V6

}

//TODO: Would be better to have SourceRanges also as IPSet instead?
//Change the code to return based on ipfamily once that is done.
func getLoadbalancerSourceRanges(filters []*localnetv1.IPFilter) []string {
	var sourceRanges []string
	for _, filter := range filters {
		if len(filter.SourceRanges) <= 0 {
			continue
		}
		sourceRanges = append(sourceRanges, filter.SourceRanges...)
	}
	return sourceRanges
}

// RequestsOnlyLocalTraffic checks if service requests OnlyLocal traffic.
func RequestsOnlyLocalTraffic(service *localnetv1.Service) bool {
	if service.Type != string(v1.ServiceTypeLoadBalancer) &&
		service.Type != string(v1.ServiceTypeNodePort) {
		return false
	}
	return service.ExternalTrafficToLocal
}

// Kernel Types used to interact with ebpf maps

// ServiceEndpoint is used to identify a service and one of its endpoint pair.
type ServiceEndpoint struct {
	Endpoint        string
	ServicePortName ServicePortName
}

// Service4Value must match 'struct lb4_service_v2' in "bpf/lib/common.h".
type Service4Value struct {
	BackendID uint32
	Count     uint16
	RevNat    uint16
	Flags     uint8
	Flags2    uint8
	Pad       pad2uint8
}

// Service4Key must match 'struct lb4_key' in "bpf/lib/common.h".
type Service4Key struct {
	Address     IPv4
	Port        Port
	BackendSlot uint16
}

// Backend4Value must match 'struct lb4_backend' in "bpf/lib/common.h".
type Backend4Value struct {
	Address IPv4
	Port    Port
	Flags   uint8
	Pad     uint8
}

type pad2uint8 [2]uint8

type IPv4 [4]byte
type Port [2]byte

type Backend4Key struct {
	ID uint32
}
