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
	"k8s.io/klog"
	"sigs.k8s.io/kpng/api/localnetv1"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
	"sigs.k8s.io/kpng/client/serviceevents"
)

func (s *Backend) updateLbIPService(svc *localnetv1.Service, serviceIP string, IPKind serviceevents.IPKind, port *localnetv1.PortMapping) {
	serviceKey := svc.Namespace + "/" + svc.Name
	s.svcs[serviceKey] = svc

	ipFamily := getIPFamily(serviceIP)
	isServiceUpdated := s.isServiceUpdated(svc)
	if !isServiceUpdated {
		s.proxiers[ipFamily].handleNewLBService(serviceKey, serviceIP, IPKind, svc, port)
	} else {
		s.proxiers[ipFamily].handleUpdatedLBService(serviceKey, serviceIP, svc, port)
	}
}

func (s *Backend) deleteLbService(svc *localnetv1.Service, serviceIP string, IPKind serviceevents.IPKind, port *localnetv1.PortMapping) {
	serviceKey := svc.Namespace + "/" + svc.Name
	s.svcs[serviceKey] = svc
	ipFamily := getIPFamily(serviceIP)
	p := s.proxiers[ipFamily]

	if IPKind == serviceevents.ClusterIP {
		p.deleteLBSvc(port, serviceIP, serviceKey)
		for _, nodeIP := range p.nodeAddresses {
			p.deleteLBSvc(port, nodeIP, serviceKey)
		}

		endPointList, isLocalEndPoint := p.deleteIPVSDestForPort(serviceKey, serviceIP, port)

		p.AddOrDelClusterIPInIPSet(serviceIP, []*localnetv1.PortMapping{port}, DeleteService)
		p.AddOrDelNodePortInIPSet(port, DeleteService)
		p.AddOrDelEndPointInIPSet(endPointList, []*localnetv1.PortMapping{port}, isLocalEndPoint, DeleteEndPoint)
	}

	if IPKind == serviceevents.LoadBalancerIP {
		p.deleteLBSvc(port, serviceIP, serviceKey)
		p.AddOrDelLbIPInIPSet(svc, serviceIP, port, DeleteService)
	}
}

func (p *proxier) handleNewLBService(serviceKey, serviceIP string, IPKind serviceevents.IPKind, svc *localnetv1.Service, port *localnetv1.PortMapping) {
	// clusterIP , nodeIPs , lb IPs need to be programmed in IPVS table
	if IPKind == serviceevents.ClusterIP {
		p.storeLBSvc(port, serviceIP, serviceKey, ClusterIPService)
		for _, nodeIP := range p.nodeAddresses {
			p.storeLBSvc(port, nodeIP, serviceKey, NodePortService)
		}

		// clusterIP , nodeIPs , lb IPs need to be programmed in ipset.
		p.AddOrDelClusterIPInIPSet(serviceIP, []*localnetv1.PortMapping{port}, AddService)
		p.AddOrDelNodePortInIPSet(port, AddService)

	}

	// LB ingress IP could be updated after service creation.
	// So LB IP needs to be programmed in IPVS, iptable once its available.
	if IPKind == serviceevents.LoadBalancerIP {
		p.storeLBSvc(port, serviceIP, serviceKey, LoadBalancerService)
		p.AddOrDelLbIPInIPSet(svc, serviceIP, port, AddService)
	}
}

func (p *proxier) handleUpdatedLBService(serviceKey, serviceIP string, svc *localnetv1.Service, port *localnetv1.PortMapping) {
	err, lbIP := p.getLbIPForIPFamily(svc)
	if err != nil {
		klog.Error(err)
	}

	//Update the service with added ports into LB tree
	p.storeLBSvc(port, serviceIP, serviceKey, ClusterIPService)
	p.storeLBSvc(port, lbIP, serviceKey, LoadBalancerService)
	for _, nodeIP := range p.nodeAddresses {
		p.storeLBSvc(port, nodeIP, serviceKey, NodePortService)
	}

	endPointList , isLocalEndPoint := p.updateIPVSDestWithPort(serviceKey, serviceIP, port)

	// Added port need to updated in clusterIP, NodePort , LB service
	// related ipsets , along with endpoint ipset.
	p.AddOrDelClusterIPInIPSet(serviceIP, []*localnetv1.PortMapping{port}, AddService)
	p.AddOrDelLbIPInIPSet(svc, lbIP, port, AddService)
	p.AddOrDelNodePortInIPSet(port, AddService)
	p.AddOrDelEndPointInIPSet(endPointList, []*localnetv1.PortMapping{port}, isLocalEndPoint, AddEndPoint)
}

func (s *Backend) handleEndPointForLBService(svcKey, key string, service *localnetv1.Service, endpoint *localnetv1.Endpoint, op Operation) {
	prefix := svcKey + "/" + key + "/"

	if op == AddEndPoint {
		// endpoint will have only one family IP, either v4/6.
		endPointIPs := endpoint.IPs.All()
		for _, ip := range endPointIPs {
			ipFamily := getIPFamily(ip)
			s.proxiers[ipFamily].SetEndPointForLBSvc(svcKey, prefix, ip, service, endpoint)
		}
	}

	if op == DeleteEndPoint {
		for _, proxier := range s.proxiers {
			proxier.DeleteEndPointForLBSvc(svcKey, prefix, service)
		}
	}
}

func (p *proxier) SetEndPointForLBSvc(svcKey, prefix, endPointIP string, service *localnetv1.Service, endpoint *localnetv1.Endpoint) {
	epInfo := endPointInfo{
		endPointIP: endPointIP,
		isLocalEndPoint: endpoint.Local,
	}

	p.endpoints.Set([]byte(prefix + endPointIP), 0, epInfo)

	// add a destination for every LB of this service. Incase of LB service ,
	// the key is just namespace+svcName unlike cluster-ip service.
	for _, lbKV := range p.lbs.GetByPrefix([]byte(svcKey)) {
		lb := lbKV.Value.(ipvsLB)
		destination := ipvsSvcDst{
			Svc:             lb.ToService(),
			Dst:             ipvsDestination(endPointIP, lb.Port, p.weight),
		}
		p.dests.Set([]byte(string(lbKV.Key)+"/"+endPointIP), 0, destination)
	}

	p.AddOrDelEndPointInIPSet([]string{endPointIP}, service.Ports, endpoint.Local, AddEndPoint)
}

func (p *proxier) DeleteEndPointForLBSvc(svcKey, prefix string, service *localnetv1.Service) {
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

func (p *proxier) AddOrDelLbIPInIPSet(svc *localnetv1.Service, lbIP string, port *localnetv1.PortMapping, op Operation) {
	var entry *ipsetutil.Entry
	entry = getIPSetEntry(lbIP, "",port)
	// add service load balancer ingressIP:Port to kubeServiceAccess ip set for the purpose of solving hairpin.
	// proxier.kubeServiceAccessSet.activeEntries.Insert(entry.String())
	// If we are proxying globally, we need to masquerade in case we cross nodes.
	// If we are proxying only locally, we can retain the source IP.
	p.setKubeLBIPSet(kubeLoadBalancerSet, entry, op)

	if svc.ExternalTrafficToLocal {
		//insert loadbalancer entry to lbIngressLocalSet if service externaltrafficpolicy=local
		p.setKubeLBIPSet(kubeLoadBalancerLocalSet, entry, op)
	}

	var isSourceRangeConfigured bool = false
	if len(svc.IPFilters) > 0 {
		for _, ip := range svc.IPFilters {
			if len (ip.SourceRanges) > 0 {
				isSourceRangeConfigured = true
			}
			for _, srcIP := range ip.SourceRanges {
				srcRangeEntry := getIPSetEntry(lbIP, srcIP, port)
				p.setKubeLBIPSet(kubeLoadBalancerSourceCIDRSet, srcRangeEntry, op)
			}
		}

		// The service firewall rules are created based on ServiceSpec.loadBalancerSourceRanges field.
		// This currently works for loadbalancers that preserves source ips.
		// For loadbalancers which direct traffic to service NodePort, the firewall rules will not apply.
		if isSourceRangeConfigured {
			p.setKubeLBIPSet(kubeLoadbalancerFWSet, entry, op)
		}
	}
}

func (p *proxier) setKubeLBIPSet(ipSetName string, entry *ipsetutil.Entry, op Operation) {
	if valid := p.ipsetList[ipSetName].validateEntry(entry); !valid {
		klog.Errorf("error adding entry :%s, to ipset:%s", entry.String(), p.ipsetList[ipSetName].Name)
		return
	}

	if op == AddService {
		p.ipsetList[ipSetName].newEntries.Insert(entry.String())
		p.updateRefCountForIPSet(ipSetName, op)
	}
	if op == DeleteService {
		p.ipsetList[ipSetName].deleteEntries.Insert(entry.String())
		p.updateRefCountForIPSet(ipSetName, op)
	}
}
