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
	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"syscall"

	"github.com/google/seesaw/ipvs"
)

type ipvsLB struct {
	IP               string
	ServiceKey       string
	ServiceType      string
	Port             *localnetv1.PortMapping
	SchedulingMethod string
}

func (lb ipvsLB) String() string {
	ba, _ := json.Marshal(lb)
	return string(ba)
}

func (lb ipvsLB) ToService() ipvs.Service {
	var port uint16
	if lb.ServiceType == ClusterIPService {
		port = uint16(lb.Port.Port)
	}
	if lb.ServiceType == NodePortService {
		port = uint16(lb.Port.NodePort)
	}
	s := ipvs.Service{
		Address:   net.ParseIP(lb.IP),
		Port:      port,
		Scheduler: lb.SchedulingMethod,
	}

	switch lb.Port.Protocol {
	case localnetv1.Protocol_TCP:
		s.Protocol = syscall.IPPROTO_TCP
	case localnetv1.Protocol_UDP:
		s.Protocol = syscall.IPPROTO_UDP
	case localnetv1.Protocol_SCTP:
		s.Protocol = syscall.IPPROTO_SCTP
	}

	return s
}

func ipvsDestination(targetIP string, port *localnetv1.PortMapping, epWeight int32) ipvs.Destination {
	return ipvs.Destination{
		Address: net.ParseIP(targetIP),
		Port:    uint16(port.TargetPort),
		Weight:  epWeight,
	}
}

type ipvsSvcDst struct {
	Svc             ipvs.Service
	Dst             ipvs.Destination
	isLocalEndPoint bool
}
