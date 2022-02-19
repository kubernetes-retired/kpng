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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

// returns a new ServicePort which abstracts a serviceInfo
func newServiceInfo(port *localnetv1.PortMapping, service *localnetv1.Service, baseInfo *BaseServiceInfo) ServicePort {
	info := &serviceInfo{BaseServiceInfo: baseInfo}

	//protoc := v1.ProtocolTCP
	//if port.Protocol == localnetv1.Protocol_UDP {
	//	protoc = v1.ProtocolUDP
	//}

	// Store the following for performance reasons.
	svcName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	svcPortName := ServicePortName{
		svcName,
		port.Name,
		port.Protocol,
	}
	//	protocol := strings.ToLower(string(info.Protocol()))
	info.serviceNameString = svcPortName.String()
	return info
}

// internal struct for string service information
type serviceInfo struct {
	// important : Dont use the proxy.BaseServiceInfo - we want to return a locavnet1 service
	*BaseServiceInfo
	targetPort             int
	externalIPs            []*externalIPInfo
	loadBalancerIngressIPs []*loadBalancerIngressInfo
	hnsID                  string
	nodePorthnsID          string
	policyApplied          bool
	remoteEndpoint         *windowsEndpoint
	hns                    HostNetworkService
	preserveDIP            bool
	localTrafficDSR        bool

	// from the other internal struct? not sure why
	serviceNameString string
}

func (svcInfo *serviceInfo) deleteAllHnsLoadBalancerPolicy() {
	// Remove the Hns Policy corresponding to this service
	hns := svcInfo.hns
	hns.deleteLoadBalancer(svcInfo.hnsID)
	svcInfo.hnsID = ""

	hns.deleteLoadBalancer(svcInfo.nodePorthnsID)
	svcInfo.nodePorthnsID = ""

	for _, externalIP := range svcInfo.externalIPs {
		hns.deleteLoadBalancer(externalIP.hnsID)
		externalIP.hnsID = ""
	}
	for _, lbIngressIP := range svcInfo.loadBalancerIngressIPs {
		hns.deleteLoadBalancer(lbIngressIP.hnsID)
		lbIngressIP.hnsID = ""
	}
}

func (svcInfo *serviceInfo) cleanupAllPolicies(proxyEndpoints *windowsEndpoint) {
	klog.V(3).InfoS("Service cleanup", "serviceInfo", svcInfo)
	// Skip the svcInfo.policyApplied check to remove all the policies
	svcInfo.deleteAllHnsLoadBalancerPolicy()
	if svcInfo.remoteEndpoint != nil {
		svcInfo.remoteEndpoint.Cleanup()
	}

	svcInfo.policyApplied = false
}
