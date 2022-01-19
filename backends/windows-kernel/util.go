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

package winkernel

import (
	"net"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

//Uses mac prefix and IPv4 address to return a mac address
//This ensures mac addresses are unique for proper load balancing
//There is a possibility of MAC collisions but this Mac address is used for remote endpoints only
//and not sent on the wire.
func conjureMac(macPrefix string, ip net.IP) string {
        if ip4 := ip.To4(); ip4 != nil {
                a, b, c, d := ip4[0], ip4[1], ip4[2], ip4[3]
                return fmt.Sprintf("%v-%02x-%02x-%02x-%02x", macPrefix, a, b, c, d)
        } else if ip6 := ip.To16(); ip6 != nil {
                a, b, c, d := ip6[15], ip6[14], ip6[13], ip6[12]
                return fmt.Sprintf("%v-%02x-%02x-%02x-%02x", macPrefix, a, b, c, d)
        }
        return "02-11-22-33-44-55"
}


func shouldSkipService(svcName types.NamespacedName, service *v1.Service) bool {
        // if ClusterIP is "None" or empty, skip proxying
        if !helper.IsServiceIPSet(service) {
                klog.V(3).InfoS("Skipping service due to clusterIP", "serviceName", svcName, "clusterIP", service.Spec.ClusterIP)
                return true
        }
        // Even if ClusterIP is set, ServiceTypeExternalName services don't get proxied
        if service.Spec.Type == v1.ServiceTypeExternalName {
                klog.V(3).InfoS("Skipping service due to Type=ExternalName", "serviceName", svcName)
                return true
        }
        return false
}
