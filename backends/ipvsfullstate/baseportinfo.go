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
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/serviceevents"
)

type EndPointInfo struct {
	endPointIP      string
	isLocalEndPoint bool
	portMap         map[string]int32
}

type ServicePortEndpointInfo struct {
	ServicePortInfo *ServicePortInfo
	EndpointsInfo   *EndPointInfo
}

type ServicePortInfo struct {
	serviceIP             string
	serviceType           string
	port                  int32
	targetPort            int32
	targetPortName        string
	nodePort              int32
	protocol              localv1.Protocol
	schedulingMethod      string
	weight                int32
	sessionAffinity       serviceevents.SessionAffinity
	stickyMaxAgeSeconds   int
	healthCheckNodePort   int
	nodeLocalExternal     bool
	nodeLocalInternal     bool
	internalTrafficPolicy *v1.ServiceInternalTrafficPolicyType
	hintsAnnotation       string
}

func NewServicePortInfo(svc *localv1.Service, port *localv1.PortMapping,
	serviceIP, serviceType,
	schedulingMethod string,
	weight int32) *ServicePortInfo {
	return &ServicePortInfo{
		serviceIP:        serviceIP,
		port:             port.Port,
		targetPort:       port.TargetPort,
		targetPortName:   port.Name,
		nodePort:         port.NodePort,
		protocol:         port.Protocol,
		schedulingMethod: schedulingMethod,
		weight:           weight,
		serviceType:      serviceType,
		sessionAffinity:  serviceevents.GetSessionAffinity(svc.SessionAffinity),
	}
}

func (b *ServicePortInfo) ServiceIP() string {
	return b.serviceIP
}

func (b *ServicePortInfo) Port() int32 {
	return b.port
}

func (b *ServicePortInfo) TargetPort() int32 {
	return b.targetPort
}

func (b *ServicePortInfo) TargetPortName() string {
	return b.targetPortName
}

func (b *ServicePortInfo) Protocol() localv1.Protocol {
	return b.protocol
}

func (b *ServicePortInfo) GetVirtualServer() ipvsLB {
	vs := ipvsLB{IP: b.serviceIP,
		SchedulingMethod: b.schedulingMethod,
		ServiceType:      b.serviceType,
		Port:             uint16(b.port),
		Protocol:         b.protocol,
		NodePort:         uint16(b.nodePort),
	}

	if b.sessionAffinity.ClientIP != nil {
		vs.Flags |= FlagPersistent
		vs.Timeout = uint32(b.sessionAffinity.ClientIP.ClientIP.TimeoutSeconds)
	}
	return vs
}
