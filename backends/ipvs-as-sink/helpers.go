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
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kpng/api/localnetv1"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
	"strconv"
)

const (
	ClusterIPService = "ClusterIP"
	NodePortService  = "NodePort"
	LoadBalancerService  = "LoadBalancer"
)

var protocolIPSetMap = map[string]string{
	ipsetutil.ProtocolTCP: kubeNodePortSetTCP,
	ipsetutil.ProtocolUDP: kubeNodePortSetUDP,
	ipsetutil.ProtocolSCTP: kubeNodePortSetSCTP,
}

type Operation int32

const (
	AddService     Operation = 0
	DeleteService  Operation = 1
	AddEndPoint    Operation = 2
	DeleteEndPoint Operation = 3
)

func asDummyIPs(ip string, ipFamily v1.IPFamily) string {
	if ipFamily == v1.IPv4Protocol {
		return  ip + "/32"
	}

	if ipFamily == v1.IPv6Protocol {
		return  ip + "/128"
	}
	return  ip + "/32"
}

func epPortSuffix(port *localnetv1.PortMapping) string {
	return port.Protocol.String() + ":" + strconv.Itoa(int(port.Port))
}
