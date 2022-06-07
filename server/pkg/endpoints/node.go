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

package endpoints

import (
	"google.golang.org/protobuf/proto"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	proxystore "sigs.k8s.io/kpng/server/pkg/proxystore"
)

const hostnameLabel = "kubernetes.io/hostname"

func ForNode(tx *proxystore.Tx, si *localnetv1.ServiceInfo, nodeName string) (endpoints []*localnetv1.EndpointInfo) {
	node := tx.GetNode(nodeName)

	if node == nil {
		// node is unknown, simulate a basic node
		node = &localnetv1.Node{
			Name: nodeName,
			Topology: &localnetv1.TopologyInfo{
				Node: nodeName,
			},
		}
	}

	svc := si.Service

	infos := make([]*localnetv1.EndpointInfo, 0)
	tx.EachEndpointOfService(svc.Namespace, svc.Name, func(info *localnetv1.EndpointInfo) {
		info = proto.Clone(info).(*localnetv1.EndpointInfo)

		info.Endpoint.Local = info.Topology.Node == nodeName

		if !info.Conditions.Ready {
			return
		}

		if hints := info.Hints; hints != nil {
			if len(hints.Zones) != 0 {
				// filter by zone
				isForNodeZone := false
				for _, z := range hints.Zones {
					if z == node.Topology.Zone {
						isForNodeZone = true
						break
					}
				}

				if !isForNodeZone {
					return
				}
			}
		}

		infos = append(infos, info)
	})

	endpoints = make([]*localnetv1.EndpointInfo, 0, len(infos))

	// select endpoints for this service

	for _, info := range infos {
		info.Endpoint.Scopes = &localnetv1.EndpointScopes{
			Internal: info.Endpoint.Local || !si.Service.InternalTrafficToLocal,
			External: info.Endpoint.Local || !si.Service.ExternalTrafficToLocal,
		}

		if info.Endpoint.Scopes.Any() {
			endpoints = append(endpoints, info)
		}
	}

	return
}
