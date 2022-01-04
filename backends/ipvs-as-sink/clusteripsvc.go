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
	"bytes"
	"sigs.k8s.io/kpng/api/localnetv1"
)

func (s *Backend) updateClusterIPService(svc *localnetv1.Service, serviceIP string, port *localnetv1.PortMapping) {
	serviceKey := svc.Namespace + "/" + svc.Name
	s.svcs[serviceKey] = svc

	ipFamily := getIPFamily(serviceIP)
	isServiceUpdated := s.isServiceUpdated(svc)
	if !isServiceUpdated {
		s.proxiers[ipFamily].handleNewClusterIPService(serviceKey, serviceIP, port)
	} else {
		s.proxiers[ipFamily].handleUpdatedClusterIPService(serviceKey, serviceIP, port)
	}
}

func (s *Backend) deleteClusterIPService(svc *localnetv1.Service, serviceIP string, port *localnetv1.PortMapping) {
	serviceKey := svc.Namespace + "/" + svc.Name
	s.svcs[serviceKey] = svc
	ipFamily := getIPFamily(serviceIP)
	p := s.proxiers[ipFamily]

	p.deleteLBSvc(port , serviceIP, serviceKey)

	endPointList , isLocalEndPoint := p.deleteIPVSDestForPort(serviceKey, serviceIP, port)

	p.AddOrDelClusterIPInIPSet(serviceIP, []*localnetv1.PortMapping{port}, DeleteService)

	p.AddOrDelEndPointInIPSet(endPointList, []*localnetv1.PortMapping{port}, isLocalEndPoint, DeleteEndPoint)
}

func (p *proxier) handleNewClusterIPService(key , clusterIP string, port *localnetv1.PortMapping) {
	//Cluster service IP is stored in LB tree
	p.storeLBSvc(port, clusterIP, key, ClusterIPService)

	//Cluster service IP needs to be programmed in ipset.
	p.AddOrDelClusterIPInIPSet(clusterIP, []*localnetv1.PortMapping{port}, AddService)
}

func (p *proxier) handleUpdatedClusterIPService(key , clusterIP string, port *localnetv1.PortMapping) {
	//Update the service with added ports into LB tree
	p.storeLBSvc(port, clusterIP, key, ClusterIPService)

	endPointList , isLocalEndPoint := p.updateIPVSDestWithPort(key, clusterIP, port)

	//Cluster service IP needs to be programmed in ipset with added ports.
	p.AddOrDelClusterIPInIPSet(clusterIP, []*localnetv1.PortMapping{port}, AddService)

	p.AddOrDelEndPointInIPSet(endPointList, []*localnetv1.PortMapping{port}, isLocalEndPoint, AddEndPoint)
}

func (s *Backend) handleEndPointForClusterIP(svcKey, key string, service *localnetv1.Service, endpoint *localnetv1.Endpoint, op Operation) {
	prefix := svcKey + "/" + key + "/"

	if op == AddEndPoint {
		// endpoint will have only one family IP, either v4/6.
		endPointIPs := endpoint.IPs.All()
		for _, ip := range endPointIPs {
			ipFamily := getIPFamily(ip)
			s.proxiers[ipFamily].SetEndPointForClusterIPSvc(svcKey, prefix, ip, service, endpoint)
		}
	}

	if op == DeleteEndPoint {
		for _, proxier := range s.proxiers {
			proxier.DeleteEndPointForClusterIPSvc(svcKey, prefix, service)
		}
	}
}

func (p *proxier) SetEndPointForClusterIPSvc(svcKey, prefix , endPointIP string, service *localnetv1.Service, endpoint *localnetv1.Endpoint) {
	epInfo := endPointInfo{
		endPointIP: endPointIP,
		isLocalEndPoint: endpoint.Local,
	}

	p.endpoints.Set([]byte(prefix + endPointIP), 0, epInfo)
	serviceIP := getServiceIPForIPFamily(p.ipFamily, service)
	// add a destination for every LB of this service
	for _, lbKV := range p.lbs.GetByPrefix([]byte(svcKey + "/" + serviceIP)) {
		lb := lbKV.Value.(ipvsLB)
		destination := ipvsSvcDst{
			Svc:             lb.ToService(),
			Dst:             ipvsDestination(endPointIP, lb.Port, p.weight),
		}
		p.dests.Set([]byte(string(lbKV.Key) + "/" + endPointIP), 0, destination)
	}

	p.AddOrDelEndPointInIPSet([]string{endPointIP}, service.Ports, endpoint.Local, AddEndPoint)
}

func (p *proxier) DeleteEndPointForClusterIPSvc(svcKey, prefix string, service *localnetv1.Service) {
	portList := service.Ports
	var endPointList []string
	var isLocalEndPoint bool

	for _, kv := range p.endpoints.GetByPrefix([]byte(prefix)) {
		epInfo := kv.Value.(endPointInfo)
		suffix := []byte("/" + epInfo.endPointIP)
		for _, destKV := range p.dests.GetByPrefix([]byte(svcKey)) {
			if bytes.HasSuffix(destKV.Key, suffix) {
				p.dests.Delete(destKV.Key)
			}
		}
		endPointList = append(endPointList, epInfo.endPointIP)
		isLocalEndPoint = epInfo.isLocalEndPoint
	}

	// remove this endpoint from the endpoints
	p.endpoints.DeleteByPrefix([]byte(prefix))

	p.AddOrDelEndPointInIPSet(endPointList, portList, isLocalEndPoint, DeleteEndPoint)
}
