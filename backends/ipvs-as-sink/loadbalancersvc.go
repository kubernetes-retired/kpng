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
	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
	"sigs.k8s.io/kpng/client/serviceevents"
)

func (s *Backend) handleLbService(svc *localnetv1.Service, serviceIP string, IPKind serviceevents.IPKind, port *localnetv1.PortMapping) {
	serviceKey := getServiceKey(svc)
	s.svcs[serviceKey] = svc
	ipFamily := getIPFamily(serviceIP)
	isServiceUpdated := s.isServiceUpdated(serviceKey)

	if !isServiceUpdated {
		s.proxiers[ipFamily].handleNewLBService(serviceKey, serviceIP, IPKind, svc, port)
	} else {
		s.proxiers[ipFamily].handleUpdatedLBService(serviceKey, serviceIP, IPKind, svc, port)
	}
}

func (s *Backend) deleteLbService(svc *localnetv1.Service, serviceIP string, IPKind serviceevents.IPKind, port *localnetv1.PortMapping) {
	serviceKey := getServiceKey(svc)
	s.svcs[serviceKey] = svc
	ipFamily := getIPFamily(serviceIP)
	p := s.proxiers[ipFamily]

	var portList []*BaseServicePortInfo
	if IPKind == serviceevents.ClusterIP {
		// --------------------------------------------------------------------------
		// ClusterIP needs to be removed from IPVS
		spKey := getServicePortKey(serviceKey, serviceIP, port)
		kv := p.servicePorts.GetByPrefix([]byte(spKey))
		portInfo := kv[0].Value.(BaseServicePortInfo)
		portList = append(portList, &portInfo)
		p.servicePorts.DeleteByPrefix([]byte(spKey))

		p.deleteVirtualServer(&portInfo)

		//Cluster service IP needs to be programmed in ipset.
		p.AddOrDelClusterIPInIPSet(&portInfo, DeleteService)
		//---------------------------------------------------------------------------

		// --------------------------------------------------------------------------
		// NodeIPs needs to be removed from IPVS
		for _, nodeIP := range p.nodeAddresses {
			spKey = getServicePortKey(serviceKey, nodeIP, port)
			kv := p.servicePorts.GetByPrefix([]byte(spKey))
			portInfo := kv[0].Value.(BaseServicePortInfo)
			portList = append(portList, &portInfo)
			p.servicePorts.DeleteByPrefix([]byte(spKey))

			p.deleteVirtualServer(&portInfo)
		}
		p.AddOrDelNodePortInIPSet(port, DeleteService)
		// --------------------------------------------------------------------------
	}

	if IPKind == serviceevents.LoadBalancerIP {
		spKey := getServicePortKey(serviceKey, serviceIP, port)
		kv := p.servicePorts.GetByPrefix([]byte(spKey))
		portInfo := kv[0].Value.(BaseServicePortInfo)
		portList = append(portList, &portInfo)

		p.servicePorts.DeleteByPrefix([]byte(spKey))

		p.deleteVirtualServer(&portInfo)
		p.AddOrDelLbIPInIPSet(svc, &portInfo, DeleteService)
	}

	epList := p.deleteRealServerForPort(serviceKey, portList)
	for _, ep := range epList {
		p.AddOrDelEndPointInIPSet(ep.endPointIP, port.Protocol.String(), port.TargetPort, ep.isLocalEndPoint, DeleteEndPoint)
	}

	portMapKey := getPortKey(serviceKey, port)
	p.deletePortFromPortMap(serviceKey, portMapKey)
}

func (p *proxier) handleNewLBService(serviceKey, serviceIP string,
	IPKind serviceevents.IPKind,
	svc *localnetv1.Service,
	port *localnetv1.PortMapping,
) {
	if _, ok := p.portMap[serviceKey]; !ok {
		p.portMap[serviceKey] = make(map[string]localnetv1.PortMapping)
	}

	portMapKey := getPortKey(serviceKey, port)
	p.portMap[serviceKey][portMapKey] = *port

	if IPKind == serviceevents.ClusterIP {
		// --------------------------------------------------------------------------
		// ClusterIP needs to be programmed in IPVS
		spKey := getServicePortKey(serviceKey, serviceIP, port)
		portInfo := NewBaseServicePortInfo(svc, port, serviceIP, ClusterIPService, p.schedulingMethod, p.weight)
		p.servicePorts.Set([]byte(spKey), 0, *portInfo)

		p.addVirtualServer(portInfo)

		//Cluster service IP needs to be programmed in ipset.
		p.AddOrDelClusterIPInIPSet(portInfo, AddService)
		//---------------------------------------------------------------------------

		// --------------------------------------------------------------------------
		// NodeIPs needs to be programmed in IPVS
		for _, nodeIP := range p.nodeAddresses {
			spKey := getServicePortKey(serviceKey, nodeIP, port)
			portInfo = NewBaseServicePortInfo(svc, port, nodeIP, NodePortService, p.schedulingMethod, p.weight)
			p.servicePorts.Set([]byte(spKey), 0, *portInfo)

			p.addVirtualServer(portInfo)
		}
		p.AddOrDelNodePortInIPSet(port, AddService)
		// --------------------------------------------------------------------------
	}

	if IPKind == serviceevents.LoadBalancerIP {
		spKey := getServicePortKey(serviceKey, serviceIP, port)
		portInfo := NewBaseServicePortInfo(svc, port, serviceIP, LoadBalancerService, p.schedulingMethod, p.weight)
		p.servicePorts.Set([]byte(spKey), 0, *portInfo)

		p.addVirtualServer(portInfo)

		p.AddOrDelLbIPInIPSet(svc, portInfo, AddService)
	}
}

func (p *proxier) handleUpdatedLBService(serviceKey, serviceIP string,
	IPKind serviceevents.IPKind,
	svc *localnetv1.Service,
	port *localnetv1.PortMapping,
) {
	err, lbIP := p.getLbIPForIPFamily(svc)
	if err != nil {
		klog.Info(err)
	}
	portMapKey := getPortKey(serviceKey, port)
	p.portMap[serviceKey][portMapKey] = *port

	var portList []*BaseServicePortInfo
	var spKey string
	if IPKind == serviceevents.ClusterIP {
		// --------------------------------------------------------------------------
		// ClusterIP needs to be programmed in IPVS
		spKey = getServicePortKey(serviceKey, serviceIP, port)
		portInfo := NewBaseServicePortInfo(svc, port, serviceIP, ClusterIPService, p.schedulingMethod, p.weight)
		p.servicePorts.Set([]byte(spKey), 0, *portInfo)
		portList = append(portList, portInfo)

		p.addVirtualServer(portInfo)

		//Cluster service IP needs to be programmed in ipset.
		p.AddOrDelClusterIPInIPSet(portInfo, AddService)
		//---------------------------------------------------------------------------

		// --------------------------------------------------------------------------
		// NodeIPs needs to be programmed in IPVS
		for _, nodeIP := range p.nodeAddresses {
			spKey := getServicePortKey(serviceKey, nodeIP, port)
			portInfo = NewBaseServicePortInfo(svc, port, nodeIP, NodePortService, p.schedulingMethod, p.weight)
			p.servicePorts.Set([]byte(spKey), 0, *portInfo)
			portList = append(portList, portInfo)

			p.addVirtualServer(portInfo)
		}
		p.AddOrDelNodePortInIPSet(port, AddService)
		// --------------------------------------------------------------------------
	}

	if IPKind == serviceevents.LoadBalancerIP {
		// LbIP needs to be programmed in IPVS
		spKey = getServicePortKey(serviceKey, lbIP, port)
		portInfo := NewBaseServicePortInfo(svc, port, lbIP, LoadBalancerService, p.schedulingMethod, p.weight)
		p.servicePorts.Set([]byte(spKey), 0, *portInfo)
		portList = append(portList, portInfo)

		p.addVirtualServer(portInfo)
		p.AddOrDelLbIPInIPSet(svc, portInfo, AddService)
	}
	endPointList := p.addRealServerForPort(serviceKey, portList)

	for _, ep := range endPointList {
		p.AddOrDelEndPointInIPSet(ep.endPointIP, port.Protocol.String(), port.TargetPort, ep.isLocalEndPoint, AddEndPoint)
	}
}

func (s *Backend) handleEndPointForLBService(svcKey, key string, endpoint *localnetv1.Endpoint, op Operation) {
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

func (p *proxier) AddOrDelLbIPInIPSet(svc *localnetv1.Service, port *BaseServicePortInfo, op Operation) {
	var entry *ipsetutil.Entry
	entry = getIPSetEntry("", port)
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
			if len(ip.SourceRanges) > 0 {
				isSourceRangeConfigured = true
			}
			for _, srcIP := range ip.SourceRanges {
				srcRangeEntry := getIPSetEntry(srcIP, port)
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
	set := p.ipsetList[ipSetName]
	if op == AddService {
		if err := set.handle.AddEntry(entry.String(), &set.IPSet, true); err != nil {
			klog.Errorf("Failed to add entry %v into ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully add entry: %v into ip set: %s", entry, set.Name)
		}
		p.updateRefCountForIPSet(ipSetName, op)
	}
	if op == DeleteService {
		if err := set.handle.DelEntry(entry.String(), set.Name); err != nil {
			klog.Errorf("Failed to delete entry: %v from ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully deleted entry: %v to ip set: %s", entry, set.Name)
		}
		p.updateRefCountForIPSet(ipSetName, op)
	}
}
