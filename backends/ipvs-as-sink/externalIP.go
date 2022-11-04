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
	"sigs.k8s.io/kpng/api/localv1"
)

func (p *proxier) handleNewExternalIP(serviceKey, externalIP, svcType string, svc *localv1.Service, port *localv1.PortMapping) {
	spKey := getServicePortKey(serviceKey, externalIP, port)
	portInfo := NewBaseServicePortInfo(svc, port, externalIP, svcType, p.schedulingMethod, p.weight)
	p.servicePorts.Set([]byte(spKey), 0, *portInfo)

	p.addVirtualServer(portInfo)

	//Cluster service IP needs to be programmed in ipset.
	p.AddOrDelExternalIPInIPSet(externalIP, portInfo, AddService)
}

func (p *proxier) handleUpdatedExternalIP(serviceKey, externalIP, svcType string, svc *localv1.Service, port *localv1.PortMapping) {
	spKey := getServicePortKey(serviceKey, externalIP, port)
	portInfo := NewBaseServicePortInfo(svc, port, externalIP, svcType, p.schedulingMethod, p.weight)
	p.servicePorts.Set([]byte(spKey), 0, *portInfo)

	//Update the service with added ports into LB tree
	p.addVirtualServer(portInfo)
	//Cluster service IP needs to be programmed in ipset with added ports.
	p.AddOrDelExternalIPInIPSet(externalIP, portInfo, AddService)

	p.addRealServerForPort(serviceKey, []*BaseServicePortInfo{portInfo})
}

func (p *proxier) AddOrDelExternalIPInIPSet(externalIP string, port *BaseServicePortInfo, op Operation) {
	entry := getIPSetEntry(externalIP, port)
	// We have to SNAT packets to external IPs.
	if valid := p.ipsetList[kubeExternalIPSet].validateEntry(entry); !valid {
		klog.Errorf("error adding entry :%s, to ipset:%s", entry.String(), p.ipsetList[kubeClusterIPSet].Name)
		return
	}
	set := p.ipsetList[kubeExternalIPSet]
	if op == AddService {
		if err := set.handle.AddEntry(entry.String(), &set.IPSet, true); err != nil {
			klog.Errorf("Failed to add entry %v into ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully add entry: %v into ip set: %s", entry, set.Name)
		}
		//Increment ref count
		p.updateRefCountForIPSet(kubeExternalIPSet, op)
	}
	if op == DeleteService {
		if err := set.handle.DelEntry(entry.String(), set.Name); err != nil {
			klog.Errorf("Failed to delete entry: %v from ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully deleted entry: %v to ip set: %s", entry, set.Name)
		}
		//Decrement ref count
		p.updateRefCountForIPSet(kubeExternalIPSet, op)
	}
}
