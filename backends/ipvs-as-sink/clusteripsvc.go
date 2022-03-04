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
	"k8s.io/klog"
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/serviceevents"
)

func (s *Backend) handleClusterIPService(svc *localnetv1.Service, serviceIP string, IPKind serviceevents.IPKind, port *localnetv1.PortMapping) {
	serviceKey := getServiceKey(svc)
	ipFamily := getIPFamily(serviceIP)

	// Handle Cluster-IP for the service
	if IPKind == serviceevents.ClusterIP {
		isServiceUpdated := s.isServiceUpdated(serviceKey)
		if !isServiceUpdated {
			s.proxiers[ipFamily].handleNewClusterIPService(serviceKey, serviceIP, svc, port)
		} else {
			s.proxiers[ipFamily].handleUpdatedClusterIPService(serviceKey, serviceIP, svc, port)
		}
	}

	// Handle External-IP for the service
	if IPKind == serviceevents.ExternalIP {
		isServiceUpdated := s.isServiceUpdated(serviceKey)
		if !isServiceUpdated {
			s.proxiers[ipFamily].handleNewExternalIP(serviceKey, serviceIP, ClusterIPService, svc, port)
		} else {
			s.proxiers[ipFamily].handleUpdatedExternalIP(serviceKey, serviceIP, ClusterIPService, svc, port)
		}
	}
}

func (s *Backend) deleteClusterIPService(svc *localnetv1.Service, serviceIP string, IPKind serviceevents.IPKind, port *localnetv1.PortMapping) {
	serviceKey := getServiceKey(svc)
	s.svcs[serviceKey] = svc
	ipFamily := getIPFamily(serviceIP)
	p := s.proxiers[ipFamily]

	spKey := getServicePortKey(serviceKey, serviceIP, port)
	kv := p.servicePorts.GetByPrefix([]byte(spKey))
	portInfo := kv[0].Value.(BaseServicePortInfo)

	epList := p.deleteRealServerForPort(serviceKey, []*BaseServicePortInfo{&portInfo})
	for _, ep := range epList {
		p.AddOrDelEndPointInIPSet(ep.endPointIP, port.Protocol.String(), port.TargetPort, ep.isLocalEndPoint, AddEndPoint)
	}

	p.deleteVirtualServer(&portInfo)

	// Delete Cluster-IP for the service
	if IPKind == serviceevents.ClusterIP {
		p.AddOrDelClusterIPInIPSet(&portInfo, DeleteService)
	}

	// Delete External-IP for the service
	if IPKind == serviceevents.ExternalIP {
		p.AddOrDelExternalIPInIPSet(serviceIP, &portInfo, DeleteService)
	}

	p.servicePorts.DeleteByPrefix([]byte(spKey))

	portMapKey := getPortKey(serviceKey, port)
	p.deletePortFromPortMap(serviceKey, portMapKey)
}

func (p *proxier) handleNewClusterIPService(serviceKey, clusterIP string, svc *localnetv1.Service, port *localnetv1.PortMapping) {
	if _, ok := p.portMap[serviceKey]; !ok {
		p.portMap[serviceKey] = make(map[string]localnetv1.PortMapping)
	}

	spKey := getServicePortKey(serviceKey, clusterIP, port)
	portInfo := NewBaseServicePortInfo(svc, port, clusterIP, ClusterIPService, p.schedulingMethod, p.weight)
	p.servicePorts.Set([]byte(spKey), 0, *portInfo)

	portMapKey := getPortKey(serviceKey, port)
	p.portMap[serviceKey][portMapKey] = *port

	p.addVirtualServer(portInfo)

	//Cluster service IP needs to be programmed in ipset.
	p.AddOrDelClusterIPInIPSet(portInfo, AddService)
}

func (p *proxier) handleUpdatedClusterIPService(serviceKey, clusterIP string, svc *localnetv1.Service, port *localnetv1.PortMapping) {
	if _, ok := p.portMap[serviceKey]; !ok {
		klog.Errorf("can't update port into non-existent service")
		return
	}

	spKey := getServicePortKey(serviceKey, clusterIP, port)
	portInfo := NewBaseServicePortInfo(svc, port, clusterIP, ClusterIPService, p.schedulingMethod, p.weight)
	p.servicePorts.Set([]byte(spKey), 0, *portInfo)

	portMapKey := getPortKey(serviceKey, port)
	p.portMap[serviceKey][portMapKey] = *port

	//Update the service with added ports into LB tree
	p.addVirtualServer(portInfo)
	//Cluster service IP needs to be programmed in ipset with added ports.
	p.AddOrDelClusterIPInIPSet(portInfo, AddService)

	epList := p.addRealServerForPort(serviceKey, []*BaseServicePortInfo{portInfo})

	for _, ep := range epList {
		p.AddOrDelEndPointInIPSet(ep.endPointIP, port.Protocol.String(), port.TargetPort, ep.isLocalEndPoint, AddEndPoint)
	}
}

func (s *Backend) handleEndPointForClusterIP(svcKey, key string, endpoint *localnetv1.Endpoint, op Operation) {
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
