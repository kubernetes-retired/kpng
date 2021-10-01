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
	"strings"

	"k8s.io/klog/v2"

	localnetv12 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/backends/ipvs/util"
)

func (s *Backend) handleNodePortService(svc *localnetv12.Service, op Operation) {
	svckey := svc.Namespace + "/" + svc.Name

	if op == AddService {
		isNewService := true
		if _, ok := s.svcs[svckey]; ok {
			isNewService = false
		}

		if isNewService {
			s.handleNewNodePortService(svckey, svc)
		} else {
			s.handleUpdatedNodePortService(svckey, svc)
		}
	}

	if op == DeleteService {
		portList := svc.Ports

		s.AddOrDelNodePortInIPSet(svc, portList, DeleteService)

		s.AddOrDelClusterIPInIPSet(svc, svc.Ports, DeleteService)
	}
}

func (s *Backend) handleNewNodePortService(key string, svc *localnetv12.Service) {
	s.svcs[key] = svc

	s.addServiceIPToKubeIPVSIntf(nil, svc)

	//Node Addresses need to be added as NodePortService
	//so that in sync(), nodePort is attached to nodeIPs.
	s.storeLBSvc(svc.Ports, s.nodeAddresses, key, NodePortService)

	//NodePort svc clusterIPs need to be added as ClusterIPService
	//so that in sync(), port is attached to clusterIP.
	s.storeLBSvc(svc.Ports, svc.IPs.All().All(), key, ClusterIPService)

	portList := svc.Ports
	s.AddOrDelNodePortInIPSet(svc, portList, AddService)

	s.AddOrDelClusterIPInIPSet(svc, svc.Ports, AddService)
}

func (s *Backend) handleUpdatedNodePortService(svckey string, svc *localnetv12.Service) {
	// update the svc
	prevSvc := s.svcs[svckey]
	s.svcs[svckey] = svc

	s.addServiceIPToKubeIPVSIntf(prevSvc, svc)

	addedPorts, removedPorts := diffInPortMapping(prevSvc, svc)

	if len(addedPorts) > 0 {
		//Node Addresses need to be added as NodePortService
		//so that in sync(), nodePort is attached to nodeIPs.
		s.storeLBSvc(addedPorts, s.nodeAddresses, svckey, NodePortService)

		//NodePort service clusterIPs need to be added as ClusterIPService
		//so that in sync(), port is attached to clusterIP.
		s.storeLBSvc(addedPorts, svc.IPs.All().All(), svckey, ClusterIPService)

		s.AddOrDelNodePortInIPSet(svc, addedPorts, AddService)

		for _, epKV := range s.endpoints.GetByPrefix([]byte(svckey + "/")) {
			endPointIP := epKV.Value.(string)

			for _, lbKV := range s.lbs.GetByPrefix([]byte(svckey + "/")) {
				lb := lbKV.Value.(ipvsLB)

				if getIPFamily(endPointIP) == getIPFamily(lb.IP) {
					for _, port := range addedPorts {
						lbKey := svckey + "/" + lb.IP + "/" + epPortSuffix(port)
						s.dests.Set([]byte(lbKey+"/"+endPointIP), 0, ipvsSvcDst{
							Svc: lb.ToService(),
							Dst: ipvsDestination(endPointIP, port, s.weight),
						})
					}
				}
			}
		}
	}

	if len(removedPorts) > 0 {
		for _, epKV := range s.endpoints.GetByPrefix([]byte(svckey + "/")) {
			endPointIP := epKV.Value.(string)

			for _, lbKV := range s.lbs.GetByPrefix([]byte(svckey + "/")) {
				lb := lbKV.Value.(ipvsLB)
				if getIPFamily(endPointIP) == getIPFamily(lb.IP) {
					for _, port := range removedPorts {
						lbKey := svckey + "/" + lb.IP + "/" + epPortSuffix(port)
						s.dests.Delete([]byte(lbKey + "/" + endPointIP))
					}
				}
			}
		}

		s.deleteLBSvc(removedPorts, s.nodeAddresses, svckey)

		s.deleteLBSvc(removedPorts, svc.IPs.All().All(), svckey)

		s.AddOrDelNodePortInIPSet(svc, removedPorts, DeleteService)
	}
}

func (s *Backend) AddOrDelNodePortInIPSet(svc *localnetv12.Service, portList []*localnetv12.PortMapping, op Operation) {
	svcIPFamily := getServiceIPFamily(svc)

	for _, port := range portList {
		var entries []*ipvs.Entry
		for _, ipFamily := range svcIPFamily {
			protocol := strings.ToLower(port.Protocol.String())
			ipsetName := protocolIPSetMap[protocol][ipFamily]
			nodePortSet := s.ipsetList[ipsetName]
			switch protocol {
			case ipvs.ProtocolTCP, ipvs.ProtocolUDP:
				entries = []*ipvs.Entry{getNodePortIPSetEntry(int(port.NodePort), protocol, ipvs.BitmapPort)}

			case ipvs.ProtocolSCTP:
				// Since hash ip:port is used for SCTP, all the nodeIPs to be used in the SCTP ipset entries.
				entries = []*ipvs.Entry{}
				for _, nodeIP := range s.nodeAddresses {
					entry := getNodePortIPSetEntry(int(port.NodePort), protocol, ipvs.HashIPPort)
					entry.IP = nodeIP
					entries = append(entries, entry)
				}
			default:
				// It should never hit
				klog.ErrorS(nil, "Unsupported protocol type", "protocol", protocol)
			}
			if nodePortSet != nil {
				for _, entry := range entries {
					if valid := nodePortSet.validateEntry(entry); !valid {
						klog.ErrorS(nil, "error adding entry to ipset", "entry", entry.String(), "ipset", nodePortSet.Name)
					}
					if op == AddService {
						nodePortSet.newEntries.Insert(entry.String())
					}
					if op == DeleteService {
						nodePortSet.deleteEntries.Insert(entry.String())
					}
				}
			}
		}
	}
}

func getNodePortIPSetEntry(port int, protocol string, ipSetType ipvs.Type) *ipvs.Entry {
	return &ipvs.Entry{
		// No need to provide ip info
		Port:     port,
		Protocol: protocol,
		SetType:  ipSetType,
	}
}

func (s *Backend) SetEndPointForNodePortSvc(svcKey, key string, endpoint *localnetv12.Endpoint) {
	prefix := svcKey + "/" + key + "/"
	service := s.svcs[svcKey]
	portList := service.Ports

	for _, endPointIP := range endpoint.IPs.All() {
		s.endpoints.Set([]byte(prefix+endPointIP), 0, endPointIP)

		// add a destination for every LB of this service
		for _, lbKV := range s.lbs.GetByPrefix([]byte(svcKey + "/")) {
			lb := lbKV.Value.(ipvsLB)

			if getIPFamily(endPointIP) == getIPFamily(lb.IP) {
				destination := ipvsSvcDst{
					Svc:             lb.ToService(),
					Dst:             ipvsDestination(endPointIP, lb.Port, s.weight),
					isLocalEndPoint: endpoint.Local,
				}
				s.dests.Set([]byte(string(lbKV.Key)+"/"+endPointIP), 0, destination)
			}
		}
	}

	s.AddOrDelEndPointInIPSet(endpoint.IPs.All(), portList, AddEndPoint)
}

func (s *Backend) DeleteEndPointForNodePortSvc(svcKey, key string) {
	prefix := []byte(svcKey + "/" + key + "/")
	service := s.svcs[svcKey]
	portList := service.Ports
	var endPointList []string

	for _, kv := range s.endpoints.GetByPrefix(prefix) {
		// remove this endpoint from the destinations if the service
		endPointIP := kv.Value.(string)
		suffix := []byte("/" + endPointIP)

		for _, destKV := range s.dests.GetByPrefix([]byte(svcKey)) {
			if bytes.HasSuffix(destKV.Key, suffix) {
				s.dests.Delete(destKV.Key)
			}
		}

		endPointList = append(endPointList, endPointIP)
	}

	// remove this endpoint from the endpoints
	s.endpoints.DeleteByPrefix(prefix)

	s.AddOrDelEndPointInIPSet(endPointList, portList, DeleteEndPoint)
}
