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

package ipvsfullsate

import (
	"encoding/json"
	IPVS "github.com/google/seesaw/ipvs"
	"net"
	"sigs.k8s.io/kpng/api/localv1"
	"syscall"
)

type ipvsSvcDst struct {
	Svc IPVS.Service
	Dst IPVS.Destination
}

type ipvsLB struct {
	IP               string
	ServiceKey       string
	ServiceType      string
	SchedulingMethod string
	Flags            IPVS.ServiceFlags
	Timeout          uint32
	Port             uint16
	NodePort         uint16
	Protocol         localv1.Protocol
}

func (lb ipvsLB) String() string {
	ba, _ := json.Marshal(lb)
	return string(ba)
}

func (lb ipvsLB) ToService() IPVS.Service {
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
	s := IPVS.Service{
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

func ipvsDestination(epInfo EndPointInfo, port *ServicePortInfo) IPVS.Destination {
	targetPort := port.TargetPort()
	if port.targetPort == 0 {
		targetPort = epInfo.portMap[port.TargetPortName()]
	}
	return IPVS.Destination{
		Address: net.ParseIP(epInfo.endPointIP),
		Port:    uint16(targetPort),
		Weight:  port.weight,
	}
}
