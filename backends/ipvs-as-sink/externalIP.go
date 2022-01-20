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
)

func (p *proxier) handleNewExternalIP(key, externalIP, svcType string, port *localnetv1.PortMapping, sessAff SessionAffinity) {
	// External IP is stored in LB tree
	p.storeLBSvc(port, sessAff, externalIP, key, svcType)

	// External IP needs to be programmed in ipset.
	p.AddOrDelExternalIPInIPSet(externalIP, []*localnetv1.PortMapping{port}, AddService)
}

func (p *proxier) handleUpdatedExternalIP(key, externalIP, svcType string, port *localnetv1.PortMapping, sessAff SessionAffinity) {
	//Update the externalIP with added ports into LB tree
	p.storeLBSvc(port, sessAff, externalIP, key, svcType)

	p.updateIPVSDestWithPort(key, externalIP, port)

	//externalIP needs to be programmed in ipset with added ports.
	p.AddOrDelExternalIPInIPSet(externalIP, []*localnetv1.PortMapping{port}, AddService)
}

func (p *proxier) AddOrDelExternalIPInIPSet(externalIP string, portList []*localnetv1.PortMapping, op Operation) {
	for _, port := range portList {
		entry := getIPSetEntry(externalIP, "", port)
		// We have to SNAT packets to external IPs.
		if valid := p.ipsetList[kubeExternalIPSet].validateEntry(entry); !valid {
			klog.Errorf("error adding entry :%s, to ipset:%s", entry.String(), p.ipsetList[kubeClusterIPSet].Name)
			return
		}
		if op == AddService {
			p.ipsetList[kubeExternalIPSet].newEntries.Insert(entry.String())
			//Increment ref count
			p.updateRefCountForIPSet(kubeExternalIPSet, op)
		}
		if op == DeleteService {
			p.ipsetList[kubeExternalIPSet].deleteEntries.Insert(entry.String())
			//Decrement ref count
			p.updateRefCountForIPSet(kubeExternalIPSet, op)
		}
	}
}
