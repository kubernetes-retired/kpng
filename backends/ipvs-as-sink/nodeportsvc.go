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
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/api/localnetv1"
)

func (s *Backend) handleNodePortService(svc *localnetv1.Service, serviceIP string, port *localnetv1.PortMapping) {
	serviceKey := getServiceKey(svc)
	s.svcs[serviceKey] = svc
	ipFamily := getIPFamily(serviceIP)

	isServiceUpdated := s.isServiceUpdated(serviceKey)
	if !isServiceUpdated {
		s.proxiers[ipFamily].handleNewNodePortService(serviceKey, serviceIP, svc, port)
	} else {
		s.proxiers[ipFamily].handleUpdatedNodePortService(serviceKey, serviceIP, svc, port)
	}
}

func (s *Backend) deleteNodePortService(svc *localnetv1.Service, serviceIP string, port *localnetv1.PortMapping) {
	serviceKey := getServiceKey(svc)
	s.svcs[serviceKey] = svc
	ipFamily := getIPFamily(serviceIP)
	p := s.proxiers[ipFamily]

	// All Node Addresses need to be deleted for port in IPVS.
	var portList []*BaseServicePortInfo
	for _, nodeIP := range p.nodeAddresses {
		spKey := getServicePortKey(serviceKey, nodeIP, port)
		kv := p.servicePorts.GetByPrefix([]byte(spKey))
		portInfo := kv[0].Value.(BaseServicePortInfo)
		portList = append(portList, &portInfo)
		p.servicePorts.DeleteByPrefix([]byte(spKey))

		p.deleteVirtualServer(&portInfo)
	}
	p.AddOrDelNodePortInIPSet(port, DeleteService)

	// ClusterIP of nodePort service needs to be deleted for port in IPVS.
	spKey := getServicePortKey(serviceKey, serviceIP, port)
	kv := p.servicePorts.GetByPrefix([]byte(spKey))
	clusterIPPortInfo := kv[0].Value.(BaseServicePortInfo)
	portList = append(portList, &clusterIPPortInfo)
	p.servicePorts.DeleteByPrefix([]byte(spKey))

	p.deleteVirtualServer(&clusterIPPortInfo)

	p.AddOrDelClusterIPInIPSet(&clusterIPPortInfo, DeleteService)

	epList := p.deleteRealServerForPort(serviceKey, portList)
	for _, ep := range epList {
		p.AddOrDelEndPointInIPSet(ep.endPointIP, port.Protocol.String(), port.TargetPort, ep.isLocalEndPoint, DeleteEndPoint)
	}

	portMapKey := getPortKey(serviceKey, port)
	p.deletePortFromPortMap(serviceKey, portMapKey)
}

func (p *proxier) handleNewNodePortService(serviceKey, clusterIP string, svc *localnetv1.Service, port *localnetv1.PortMapping) {
	if _, ok := p.portMap[serviceKey]; !ok {
		p.portMap[serviceKey] = make(map[string]localnetv1.PortMapping)
	}

	portMapKey := getPortKey(serviceKey, port)
	p.portMap[serviceKey][portMapKey] = *port

	// All Node Addresses need to be added as virtual servers in IPVS.
	for _, nodeIP := range p.nodeAddresses {
		spKey := getServicePortKey(serviceKey, nodeIP, port)
		portInfo := NewBaseServicePortInfo(svc, port, nodeIP, NodePortService, p.schedulingMethod, p.weight)
		p.servicePorts.Set([]byte(spKey), 0, *portInfo)

		p.addVirtualServer(portInfo)
	}
	// Only here direct port obj is used instead of portInfo
	p.AddOrDelNodePortInIPSet(port, AddService)

	// ClusterIP of nodePort service needs to be added as virtual servers in IPVS.
	spKey := getServicePortKey(serviceKey, clusterIP, port)
	portInfo := NewBaseServicePortInfo(svc, port, clusterIP, ClusterIPService, p.schedulingMethod, p.weight)
	p.servicePorts.Set([]byte(spKey), 0, *portInfo)

	p.addVirtualServer(portInfo)

	p.AddOrDelClusterIPInIPSet(portInfo, AddService)
}

func (p *proxier) handleUpdatedNodePortService(serviceKey, clusterIP string, svc *localnetv1.Service, port *localnetv1.PortMapping) {
	if _, ok := p.portMap[serviceKey]; !ok {
		klog.Errorf("can't update port into non-existent service")
		return
	}

	portMapKey := getPortKey(serviceKey, port)
	p.portMap[serviceKey][portMapKey] = *port

	// --------------------------------------------------------------------------
	// All Node Addresses need to be updated with new port as virtual servers in IPVS.
	var portList []*BaseServicePortInfo
	for _, nodeIP := range p.nodeAddresses {
		spKey := getServicePortKey(serviceKey, nodeIP, port)
		portInfo := NewBaseServicePortInfo(svc, port, nodeIP, NodePortService, p.schedulingMethod, p.weight)
		p.servicePorts.Set([]byte(spKey), 0, *portInfo)
		portList = append(portList, portInfo)

		p.addVirtualServer(portInfo)
	}
	p.AddOrDelNodePortInIPSet(port, AddService)
	// --------------------------------------------------------------------------

	// --------------------------------------------------------------------------
	// ClusterIP of nodePort service needs to be updated with new port as virtual servers in IPVS.
	spKey := getServicePortKey(serviceKey, clusterIP, port)
	portInfo := NewBaseServicePortInfo(svc, port, clusterIP, ClusterIPService, p.schedulingMethod, p.weight)
	p.servicePorts.Set([]byte(spKey), 0, *portInfo)

	portList = append(portList, portInfo)
	p.addVirtualServer(portInfo)

	p.AddOrDelClusterIPInIPSet(portInfo, AddService)
	// --------------------------------------------------------------------------

	endPointList := p.addRealServerForPort(serviceKey, portList)

	for _, ep := range endPointList {
		p.AddOrDelEndPointInIPSet(ep.endPointIP, port.Protocol.String(), port.TargetPort, ep.isLocalEndPoint, AddEndPoint)
	}
}

func (s *Backend) handleEndPointForNodePortService(svcKey, key string, endpoint *localnetv1.Endpoint, op Operation) {
	prefix := svcKey + "/" + key + "/"
	if op == AddEndPoint {
		// endpoint will have only one family IP, either v4/6.
		endPointIPs := endpoint.IPs.All()
		for _, ip := range endPointIPs {
			ipFamily := getIPFamily(ip)
			s.proxiers[ipFamily].addRealServer(svcKey, prefix, ip, endpoint)
		}
	}

	if op == DeleteEndPoint {
		for _, proxier := range s.proxiers {
			proxier.deleteRealServer(svcKey, prefix)
		}
	}
}
