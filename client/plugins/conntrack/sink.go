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

package conntrack

import (
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/localsink"
)

type Sink struct {
	localsink.Config
	services     map[string]*localv1.Service
	endpoints    map[string]map[string]*localv1.Endpoint
	staleFlows   []Flow
	staleIPPorts []IPPort
}

func NewSink() *Sink {
	return &Sink{
		services:  make(map[string]*localv1.Service),
		endpoints: make(map[string]map[string]*localv1.Endpoint),
	}
}

func (ps *Sink) Reset() {
}

func (ps *Sink) Setup() {
}

func (ps *Sink) SetService(svc *localv1.Service) {
	ps.services[svc.Namespace+"/"+svc.Name] = svc
}

func (ps *Sink) DeleteService(namespace, name string) {
	delete(ps.services, namespace+"/"+name)
}

func (ps *Sink) SetEndpoint(namespace, serviceName, key string, endpoint *localv1.Endpoint) {
	service, _ := ps.services[namespace+"/"+serviceName]
	eps := len(ps.endpoints[namespace+"/"+serviceName])
	allIPs := service.IPs.All().All()
	if eps == 0 {
		if service.Type == "NodePort" {
			allIPs = append(allIPs, "node")
		}

		for _, svcIP := range allIPs {
			for _, port := range service.Ports {
				targetPort := port.Port
				if svcIP == "node" {
					targetPort = port.NodePort
				}

				if port.Port == 0 {
					continue
				}

				ps.staleIPPorts = append(ps.staleIPPorts, IPPort{Protocol: port.Protocol, DnatIP: svcIP, Port: targetPort})
			}
		}
	}
	if ps.endpoints[namespace+"/"+serviceName] == nil {
		ps.endpoints[namespace+"/"+serviceName] = make(map[string]*localv1.Endpoint)
	}
	ps.endpoints[namespace+"/"+serviceName][key] = endpoint
}

func (ps *Sink) DeleteEndpoint(namespace, serviceName, key string) {
	service, _ := ps.services[namespace+"/"+serviceName]
	ep, _ := ps.endpoints[namespace+"/"+serviceName][key]
	var targetPort int32
	for _, svcIP := range service.IPs.All().All() {
		for _, port := range service.Ports {
			for _, epIP := range ep.IPs.All() {
				targetPort = port.Port
				if port.Port == 0 {
					p, err := ep.PortMapping(port)
					if err != nil {
						klog.V(1).InfoS("failed to map port", "err", err)
						continue
					}
					targetPort = p
				}
				flow := Flow{
					IPPort:     IPPort{Protocol: port.Protocol, DnatIP: svcIP, Port: targetPort},
					EndpointIP: epIP,
					TargetPort: targetPort,
				}
				ps.staleFlows = append(ps.staleFlows, flow)
			}
		}
	}
	delete(ps.endpoints[namespace+"/"+serviceName], key)
}

func (s *Sink) Sync() {
	for _, ipPort := range s.staleIPPorts {
		cleanupIPPortEntries(ipPort)
	}
	for _, flow := range s.staleFlows {
		cleanupFlowEntries(flow)
	}
	s.staleIPPorts = nil
	s.staleFlows = nil
}
