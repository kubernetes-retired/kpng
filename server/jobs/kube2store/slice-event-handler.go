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

package kube2store

import (
	"sort"

	"k8s.io/klog/v2"
	discovery "k8s.io/api/discovery/v1"

	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/api/globalv1"
	"sigs.k8s.io/kpng/server/proxystore"
)

const hostNameLabel = "kubernetes.io/hostname"

type sliceEventHandler struct{ eventHandler }

func serviceNameFrom(eps *discovery.EndpointSlice) string {
	if eps.Labels == nil {
		return ""
	}
	return eps.Labels[discovery.LabelServiceName]
}

func (h sliceEventHandler) OnAdd(obj interface{}) {
	eps := obj.(*discovery.EndpointSlice)
	serviceName := serviceNameFrom(eps)
	if serviceName == "" {
		// no name => not associated with a service => ignore
		return
	}

	// compute endpoints
	infos := make([]*globalv1.EndpointInfo, 0, len(eps.Endpoints))

	for _, sliceEndpoint := range eps.Endpoints {
		info := &globalv1.EndpointInfo{
			Namespace:   eps.Namespace,
			ServiceName: serviceName,
			SourceName:  eps.Name,
			Endpoint:    &localv1.Endpoint{},
			Conditions:  &globalv1.EndpointConditions{},
			Topology:    &globalv1.TopologyInfo{},
		}

		if t := sliceEndpoint.TargetRef; t != nil && t.Kind == "Pod" {
			info.PodName = t.Name
		}

		if h := sliceEndpoint.Hostname; h != nil {
			info.Endpoint.Hostname = *h
		}

		if n := sliceEndpoint.NodeName; n != nil {
			info.Topology.Node = *n
		}
		if z := sliceEndpoint.Zone; z != nil {
			info.Topology.Zone = *z
		}

		if hints := sliceEndpoint.Hints; hints != nil {
			info.Hints = &globalv1.TopologyHints{
				Zones: make([]string, 0, len(hints.ForZones)),
			}

			for _, z := range hints.ForZones {
				info.Hints.Zones = append(info.Hints.Zones, z.Name)
			}
			sort.Strings(info.Hints.Zones) // stable zone order
		}

		if r := sliceEndpoint.Conditions.Ready; r != nil && *r {
			info.Conditions.Ready = true
		}

		for _, addr := range sliceEndpoint.Addresses {
			info.Endpoint.AddAddress(addr)
		}

		ports := make([]*localv1.PortName, 0, len(eps.Ports))
		for _, port := range eps.Ports {
			ports = append(ports, &localv1.PortName{Name: *port.Name, Port: *port.Port})
		}
		info.Endpoint.PortOverrides = ports

		infos = append(infos, info)
	}

	h.s.Update(func(tx *proxystore.Tx) {
		tx.SetEndpointsOfSource(eps.Namespace, eps.Name, infos)
		h.updateSync(proxystore.Endpoints, tx)

		if log := klog.V(3); log.Enabled() {
			log.Info("endpoints of ", eps.Namespace, "/", serviceName, ":")
			tx.EachEndpointOfService(eps.Namespace, serviceName, func(ei *globalv1.EndpointInfo) {
				log.Info("- ", ei.Endpoint.IPs, " | topo: ", ei.Topology)
			})
		}
	})
}

func (h sliceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}

func (h sliceEventHandler) OnDelete(oldObj interface{}) {
	eps := oldObj.(*discovery.EndpointSlice)

	h.s.Update(func(tx *proxystore.Tx) {
		tx.DelEndpointsOfSource(eps.Namespace, eps.Name)
		h.updateSync(proxystore.Endpoints, tx)
	})
}
