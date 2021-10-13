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

func (s *Backend) handleClusterIPService(svc *localnetv1.Service, op Operation) {
	key := svc.Namespace + "/" + svc.Name

	if op == AddService {
		isNewService := true
		if _, ok := s.svcs[key]; ok {
			isNewService = false
		}

		if isNewService {
			s.handleNewClusterIPService(key, svc)
		} else {
			s.handleUpdatedClusterIPService(key, svc)
		}
	}

	if op == DeleteService {
		s.AddOrDelClusterIPInIPSet(svc, svc.Ports, DeleteService)
	}
}

func (s *Backend) handleNewClusterIPService(key string, svc *localnetv1.Service) {
	s.svcs[key] = svc

	//Cluster service IP is added to kube-ipvs0 interface
	s.addServiceIPToKubeIPVSIntf(nil, svc)

	//Cluster service IP is stored in LB tree
	s.storeLBSvc(svc.Ports, svc.IPs.All().All(), key, ClusterIPService)

	//Cluster service IP needs to be programmed in ipset.
	s.AddOrDelClusterIPInIPSet(svc, svc.Ports, AddService)
}

func (s *Backend) handleUpdatedClusterIPService(key string, svc *localnetv1.Service) {
	// update the svc
	prevSvc := s.svcs[key]
	s.svcs[key] = svc

	//Updated Cluster service IP is added/removed from kube-ipvs0 interface
	s.addServiceIPToKubeIPVSIntf(prevSvc, svc)

	// When existing service gets updated with new port/protocol, endpoints
	// behind it also needs to be updated into tree, so that they are handled in sync().
	addedPorts, removedPorts := diffInPortMapping(prevSvc, svc)

	if len(addedPorts) > 0 {
		//Update the service with added ports into LB tree
		s.storeLBSvc(addedPorts, svc.IPs.All().All(), key, ClusterIPService)
		var endPointList []string
		var isLocalEndPoint bool

		for _, epKV := range s.endpoints.GetByPrefix([]byte(key + "/")) {
			epInfo := epKV.Value.(endPointInfo)
			endPointList = append(endPointList, epInfo.endPointIP)
			svcIP := getServiceIP(epInfo.endPointIP, svc)
			isLocalEndPoint = epInfo.isLocalEndPoint

			for _, port := range addedPorts {
				lbKey := key + "/" + svcIP + "/" + epPortSuffix(port)
				ipvslb := s.lbs.GetByPrefix([]byte(lbKey))

				s.dests.Set([]byte(lbKey+"/"+epInfo.endPointIP), 0, ipvsSvcDst{
					Svc: ipvslb[0].Value.(ipvsLB).ToService(),
					Dst: ipvsDestination(epInfo.endPointIP, port, s.weight),
				})
			}
		}

		//Cluster service IP needs to be programmed in ipset with added ports.
		s.AddOrDelClusterIPInIPSet(svc, addedPorts, AddService)

		s.AddOrDelEndPointInIPSet(endPointList, addedPorts, isLocalEndPoint, AddEndPoint)
	}

	// When existing service gets updated with deletion of port/protocol,
	// endpoint behind it needs to be removed from tree.
	if len(removedPorts) > 0 {
		s.deleteLBSvc(removedPorts, svc.IPs.All().All(), key)
		var endPointList []string
		var isLocalEndPoint bool

		for _, epKV := range s.endpoints.GetByPrefix([]byte(key + "/")) {
			epInfo := epKV.Value.(endPointInfo)
			endPointList = append(endPointList, epInfo.endPointIP)
			svcIP := getServiceIP(epInfo.endPointIP, svc)
			isLocalEndPoint = epInfo.isLocalEndPoint

			for _, port := range removedPorts {
				lbKey := key + "/" + svcIP + "/" + epPortSuffix(port)
				s.dests.Delete([]byte(lbKey + "/" + epInfo.endPointIP))
			}
		}

		s.AddOrDelClusterIPInIPSet(svc, removedPorts, DeleteService)

		s.AddOrDelEndPointInIPSet(endPointList, removedPorts, isLocalEndPoint, DeleteEndPoint)
	}
}

func (s *Backend) SetEndPointForClusterIPSvc(svcKey, key string, endpoint *localnetv1.Endpoint) {
	prefix := svcKey + "/" + key + "/"
	service := s.svcs[svcKey]
	portList := service.Ports

	for _, endPointIP := range endpoint.IPs.All() {
		epInfo := endPointInfo{
			endPointIP: endPointIP,
			isLocalEndPoint: endpoint.Local,
		}
		s.endpoints.Set([]byte(prefix+endPointIP), 0, epInfo)
		svcIP := getServiceIP(endPointIP, service)

		// add a destination for every LB of this service
		for _, lbKV := range s.lbs.GetByPrefix([]byte(svcKey + "/" + svcIP)) {
			lb := lbKV.Value.(ipvsLB)
			destination := ipvsSvcDst{
				Svc:             lb.ToService(),
				Dst:             ipvsDestination(endPointIP, lb.Port, s.weight),
			}
			s.dests.Set([]byte(string(lbKV.Key)+"/"+endPointIP), 0, destination)
		}
	}

	s.AddOrDelEndPointInIPSet(endpoint.IPs.All(), portList, endpoint.Local, AddEndPoint)
}

func (s *Backend) DeleteEndPointForClusterIPSvc(svcKey, key string) {
	prefix := []byte(svcKey + "/" + key + "/")
	service := s.svcs[svcKey]
	portList := service.Ports
	var endPointList []string
	var isLocalEndPoint bool

	for _, kv := range s.endpoints.GetByPrefix(prefix) {
		epInfo := kv.Value.(endPointInfo)
		suffix := []byte("/" + epInfo.endPointIP)

		for _, destKV := range s.dests.GetByPrefix([]byte(svcKey)) {
			if bytes.HasSuffix(destKV.Key, suffix) {
				s.dests.Delete(destKV.Key)
			}
		}
		endPointList = append(endPointList, epInfo.endPointIP)
		isLocalEndPoint = epInfo.isLocalEndPoint
	}

	// remove this endpoint from the endpoints
	s.endpoints.DeleteByPrefix(prefix)

	s.AddOrDelEndPointInIPSet(endPointList, portList, isLocalEndPoint, DeleteEndPoint)
}
