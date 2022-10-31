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
	"encoding/json"
	"net"
	"syscall"

	"sigs.k8s.io/kpng/api/localv1"

	"github.com/google/seesaw/ipvs"
)

type ipvsSvcDst struct {
	Svc ipvs.Service
	Dst ipvs.Destination
}

type ipvsLB struct {
	IP               string
	ServiceKey       string
	ServiceType      string
	SchedulingMethod string
	Flags            ipvs.ServiceFlags
	Timeout          uint32
	Port             uint16
	NodePort         uint16
	Protocol         localv1.Protocol
}

func (lb ipvsLB) String() string {
	ba, _ := json.Marshal(lb)
	return string(ba)
}

func (lb ipvsLB) ToService() ipvs.Service {
	var port uint16
	if lb.ServiceType == ClusterIPService {
		port = lb.Port
	}
	if lb.ServiceType == NodePortService {
		port = lb.NodePort
	}
	if lb.ServiceType == LoadBalancerService {
		port = lb.Port
	}
	s := ipvs.Service{
		Address:   net.ParseIP(lb.IP),
		Port:      port,
		Scheduler: lb.SchedulingMethod,
		Flags:     lb.Flags,
		Timeout:   lb.Timeout,
	}

	switch lb.Protocol {
	case localv1.Protocol_TCP:
		s.Protocol = syscall.IPPROTO_TCP
	case localv1.Protocol_UDP:
		s.Protocol = syscall.IPPROTO_UDP
	case localv1.Protocol_SCTP:
		s.Protocol = syscall.IPPROTO_SCTP
	}

	return s
}

func ipvsDestination(epInfo endPointInfo, port *BaseServicePortInfo) ipvs.Destination {
	targetPort := port.TargetPort()
	if port.targetPort == 0 {
		targetPort = epInfo.portMap[port.TargetPortName()]
	}
	return ipvs.Destination{
		Address: net.ParseIP(epInfo.endPointIP),
		Port:    uint16(targetPort),
		Weight:  port.weight,
	}
}
