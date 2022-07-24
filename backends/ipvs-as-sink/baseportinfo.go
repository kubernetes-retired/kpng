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
	"sigs.k8s.io/kpng/client/serviceevents"
)

type BaseServicePortInfo struct {
	serviceIP             string
	serviceType           string
	port                  int32
	targetPort            int32
	targetPortName        string
	nodePort              int32
	protocol              localnetv1.Protocol
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

func NewBaseServicePortInfo(svc *localnetv1.Service, port *localnetv1.PortMapping,
	serviceIP, serviceType,
	schedulingMethod string,
	weight int32) *BaseServicePortInfo {
	return &BaseServicePortInfo{
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

func (b *BaseServicePortInfo) ServiceIP() string {
	return b.serviceIP
}

func (b *BaseServicePortInfo) Port() int32 {
	return b.port
}

func (b *BaseServicePortInfo) TargetPort() int32 {
	return b.targetPort
}

func (b *BaseServicePortInfo) TargetPortName() string {
	return b.targetPortName
}

func (b *BaseServicePortInfo) Protocol() localnetv1.Protocol {
	return b.protocol
}

func (b *BaseServicePortInfo) GetVirtualServer() ipvsLB {
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

func (b *BaseServicePortInfo) SetSessionAffinity(sa serviceevents.SessionAffinity) {
	b.sessionAffinity = sa
}

func (b *BaseServicePortInfo) ResetSessionAffinity() {
	b.sessionAffinity = serviceevents.SessionAffinity{}
}
