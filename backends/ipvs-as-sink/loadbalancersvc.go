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
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/kpng/api/localnetv1"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
)

func (s *Backend) handleLBService(svc *localnetv1.Service, op Operation) {
	key := svc.Namespace + "/" + svc.Name

	if op == AddService {
		isNewService := true
		if _, ok := s.svcs[key]; ok {
			isNewService = false
		}

		if isNewService {
			s.handleNewLBService(key, svc)
		} else {
			s.handleUpdatedLBService(key, svc)
		}
	}

	if op == DeleteService {
		s.AddOrDelClusterIPInIPSet(svc, svc.Ports, DeleteService)
		s.AddOrDelLbIPInIPSet(svc, svc.Ports, DeleteService)
		s.AddOrDelNodePortInIPSet(svc, svc.Ports, DeleteService)
	}
}

func (s *Backend) handleNewLBService(key string, svc *localnetv1.Service) {
	s.svcs[key] = svc

	//ClusterIP and  Loadbalancer IP is added to kube-ipvs0 interface
	s.addServiceIPToKubeIPVSIntf(nil, svc)

	//For LB service, clusterIP , nodeIPs , loadbalancerIPs need to be programmed
	// in IPVS table
	s.storeLBSvc(svc.Ports, svc.IPs.ClusterIPs.All(), key, ClusterIPService)
	s.storeLBSvc(svc.Ports, svc.IPs.LoadBalancerIPs.All(), key, LoadBalancerService)
	s.storeLBSvc(svc.Ports, s.nodeAddresses, key, NodePortService)


	//For LB service, clusterIP , nodeIPs , loadbalancerIPs need to be programmed
	// in ipset.
	s.AddOrDelClusterIPInIPSet(svc, svc.Ports, AddService)
	s.AddOrDelLbIPInIPSet(svc, svc.Ports, AddService)
	s.AddOrDelNodePortInIPSet(svc, svc.Ports, AddService)
}

func (s *Backend) handleUpdatedLBService(key string, svc *localnetv1.Service) {
	// update the svc
	prevSvc := s.svcs[key]
	s.svcs[key] = svc

	//Updated Cluster IP is added/removed from kube-ipvs0 interface
	s.addServiceIPToKubeIPVSIntf(prevSvc, svc)

	// When existing service gets updated with new port/protocol, endpoints
	// behind it also needs to be updated into tree, so that they are handled in sync().
	addedPorts, removedPorts := diffInPortMapping(prevSvc, svc)

	if len(addedPorts) > 0 {
		//Update the service with added ports into LB tree
		s.storeLBSvc(addedPorts, svc.IPs.ClusterIPs.All(), key, ClusterIPService)
		s.storeLBSvc(addedPorts, svc.IPs.LoadBalancerIPs.All(), key, LoadBalancerService)
		s.storeLBSvc(addedPorts, s.nodeAddresses, key, NodePortService)

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

		//Added port info need to updated in clusterIP, NodePort , LB service
		// related ipsets , along with endpoint ipset.
		s.AddOrDelClusterIPInIPSet(svc, addedPorts, AddService)
		s.AddOrDelLbIPInIPSet(svc, addedPorts, AddService)
		s.AddOrDelNodePortInIPSet(svc, addedPorts, AddService)
		s.AddOrDelEndPointInIPSet(endPointList, addedPorts, isLocalEndPoint, AddEndPoint)
	}

	// When existing service gets updated with deletion of port/protocol,
	// endpoint behind it needs to be removed from tree.
	if len(removedPorts) > 0 {
		s.deleteLBSvc(removedPorts, svc.IPs.ClusterIPs.All(), key)
		s.deleteLBSvc(removedPorts, svc.IPs.LoadBalancerIPs.All(), key)
		s.deleteLBSvc(removedPorts, s.nodeAddresses, key)

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

		//Removed port info need to removed in clusterIP, NodePort , LB service
		// related ipsets , along with endpoint ipset.
		s.AddOrDelClusterIPInIPSet(svc, removedPorts, DeleteService)
		s.AddOrDelLbIPInIPSet(svc, removedPorts, DeleteService)
		s.AddOrDelNodePortInIPSet(svc, removedPorts, DeleteService)
		s.AddOrDelEndPointInIPSet(endPointList, removedPorts, isLocalEndPoint, DeleteEndPoint)
	}
}

func (s *Backend) SetEndPointForLBSvc(svcKey, key string, endpoint *localnetv1.Endpoint) {
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

func (s *Backend) DeleteEndPointForLBSvc(svcKey, key string) {
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

func (s *Backend) AddOrDelLbIPInIPSet(svc *localnetv1.Service, portList []*localnetv1.PortMapping, op Operation) {
	svcIPFamily := getLBServiceIPFamily(svc)
	var ipSetName string
	var entry *ipsetutil.Entry

	for _, port := range portList {
		for _, ipFamily := range svcIPFamily {
			var lbIP string
			if ipFamily == v1.IPv4Protocol {
				lbIP = svc.IPs.LoadBalancerIPs.V4[0]
			}
			if ipFamily == v1.IPv6Protocol {
				lbIP = svc.IPs.LoadBalancerIPs.V6[0]
			}

			entry = getIPSetEntry(lbIP, "",port)
			// add service load balancer ingressIP:Port to kubeServiceAccess ip set for the purpose of solving hairpin.
			// proxier.kubeServiceAccessSet.activeEntries.Insert(entry.String())
			// If we are proxying globally, we need to masquerade in case we cross nodes.
			// If we are proxying only locally, we can retain the source IP.
			ipSetName = loadbalancerIPSetMap[ipFamily]
			s.setKubeLBIPSet(ipSetName, entry, op)

			if svc.ExternalTrafficToLocal {
				//insert loadbalancer entry to lbIngressLocalSet if service externaltrafficpolicy=local
				ipSetName = loadbalancerLocalSetMap[ipFamily]
				s.setKubeLBIPSet(ipSetName, entry, op)
			}

			var isSourceRangeConfigured bool = false
			if len(svc.IPFilters) > 0 {
				for _, ip := range svc.IPFilters {
					if len (ip.SourceRanges) > 0 {
						isSourceRangeConfigured = true
					}
					for _, srcIP := range ip.SourceRanges {
						srcRangeEntry := getIPSetEntry(lbIP, srcIP, port)
						ipSetName = loadbalancerSourceCIDRSetMap[ipFamily]
						s.setKubeLBIPSet(ipSetName, srcRangeEntry, op)
					}
				}

				// The service firewall rules are created based on ServiceSpec.loadBalancerSourceRanges field.
				// This currently works for loadbalancers that preserves source ips.
				// For loadbalancers which direct traffic to service NodePort, the firewall rules will not apply.
				if isSourceRangeConfigured {
					ipSetName = loadbalancerFWSetMap[ipFamily]
					s.setKubeLBIPSet(ipSetName, entry, op)
				}
			}

		}
	}
}

func (s *Backend) setKubeLBIPSet(ipSetName string, entry *ipsetutil.Entry, op Operation) {
	if valid := s.ipsetList[ipSetName].validateEntry(entry); !valid {
		klog.Errorf("error adding entry :%s, to ipset:%s", entry.String(), s.ipsetList[ipSetName].Name)
		return
	}
	if op == AddService {
		s.ipsetList[ipSetName].newEntries.Insert(entry.String())
	}
	if op == DeleteService {
		s.ipsetList[ipSetName].deleteEntries.Insert(entry.String())
	}
}

func getLBServiceIPFamily(svc *localnetv1.Service) []v1.IPFamily {
	var svcIPFamily []v1.IPFamily
	if svc.IPs.LoadBalancerIPs == nil {
		return svcIPFamily
	}
	if len(svc.IPs.LoadBalancerIPs.V4) > 0 && len(svc.IPs.LoadBalancerIPs.V6) == 0 {
		svcIPFamily = append(svcIPFamily, v1.IPv4Protocol)
	}

	if len(svc.IPs.LoadBalancerIPs.V6) > 0 && len(svc.IPs.LoadBalancerIPs.V4) == 0 {
		svcIPFamily = append(svcIPFamily, v1.IPv6Protocol)
	}

	if len(svc.IPs.LoadBalancerIPs.V4) > 0 && len(svc.IPs.LoadBalancerIPs.V6) > 0 {
		svcIPFamily = append(svcIPFamily, v1.IPv4Protocol, v1.IPv6Protocol)
	}
	return svcIPFamily
}