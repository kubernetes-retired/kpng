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

package ipvssink

import (
	"strconv"

	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/kpng/api/localnetv1"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
)

const (
	ClusterIPService = "ClusterIP"
	NodePortService  = "NodePort"
	LoadBalancerService  = "LoadBalancer"
)

var loopBackIPSetMap = map[v1.IPFamily]string{
	v1.IPv4Protocol: kubeLoopBackIPv4Set,
	v1.IPv6Protocol: kubeLoopBackIPv6Set,
}

var clusterIPSetMap = map[v1.IPFamily]string{
	v1.IPv4Protocol: kubeClusterIPv4Set,
	v1.IPv6Protocol: kubeClusterIPv6Set,
}

var protocolIPSetMap = map[string]map[v1.IPFamily]string{
	ipsetutil.ProtocolTCP: {
		v1.IPv4Protocol: kubeNodePortIPv4SetTCP,
		v1.IPv6Protocol: kubeNodePortIPv6SetTCP,
	},
	ipsetutil.ProtocolUDP: {
		v1.IPv4Protocol: kubeNodePortIPv4SetUDP,
		v1.IPv6Protocol: kubeNodePortIPv6SetUDP,
	},
	ipsetutil.ProtocolSCTP: {
		v1.IPv4Protocol: kubeNodePortIPv4SetSCTP,
		v1.IPv6Protocol: kubeNodePortIPv6SetSCTP,
	},
}

var loadbalancerIPSetMap = map[v1.IPFamily]string{
	v1.IPv4Protocol: kubeLoadBalancerIPv4Set,
	v1.IPv6Protocol: kubeLoadBalancerIPv6Set,
}

var loadbalancerLocalSetMap = map[v1.IPFamily]string{
	v1.IPv4Protocol: kubeLoadBalancerLocalIPv4Set,
	v1.IPv6Protocol: kubeLoadBalancerLocalIPv6Set,
}

var loadbalancerSourceCIDRSetMap = map[v1.IPFamily]string{
	v1.IPv4Protocol: kubeLoadBalancerSourceCIDRIPv4Set,
	v1.IPv6Protocol: kubeLoadBalancerSourceCIDRIPv6Set,
}

var loadbalancerFWSetMap = map[v1.IPFamily]string{
	v1.IPv4Protocol: kubeLoadbalancerFWIPv4Set,
	v1.IPv6Protocol: kubeLoadbalancerFWIPv6Set,
}

type Operation int32

const (
	AddService     Operation = 0
	DeleteService  Operation = 1
	AddEndPoint    Operation = 2
	DeleteEndPoint Operation = 3
)

func asDummyIPs(set *localnetv1.IPSet) (ips []string) {
	ips = make([]string, 0, len(set.V4)+len(set.V6))

	for _, ip := range set.V4 {
		ips = append(ips, ip+"/32")
	}
	for _, ip := range set.V6 {
		ips = append(ips, ip+"/128")
	}

	return
}

func epPortSuffix(port *localnetv1.PortMapping) string {
	return port.Protocol.String() + ":" + strconv.Itoa(int(port.Port))
}

func diffInPortMapping(previous, current *localnetv1.Service) (added, removed []*localnetv1.PortMapping) {
	for _, p1 := range previous.Ports {
		found := false
		for _, p2 := range current.Ports {
			if p1.Name == p2.Name && p1.Port == p2.Port && p1.Protocol == p2.Protocol {
				found = true
				break
			}
		}

		if !found {
			removed = append(removed, p1)
		}
	}

	for _, p1 := range current.Ports {
		found := false
		for _, p2 := range previous.Ports {
			if p1.Name == p2.Name && p1.Port == p2.Port && p1.Protocol == p2.Protocol {
				found = true
				break
			}
		}

		if !found {
			added = append(added, p1)
		}
	}
	return
}
