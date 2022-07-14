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

func ForNode(tx *proxystore.Tx, si *localnetv1.ServiceInfo, nodeName string) (internalEndpoints, externalEndpoints []*localnetv1.EndpointInfo) {
	node := tx.GetNode(nodeName)

	var labels map[string]string

	if node == nil || len(node.Labels) == 0 {
		// node is unknown or has no labels, simulate basic node label
		labels = map[string]string{}
	} else {
		labels = node.Labels
	}

	if labels[hostnameLabel] == "" {
		// ensure we have the hostname even if it's filtered
		labels[hostnameLabel] = nodeName
	}

	svc := si.Service

	infos := make([]*localnetv1.EndpointInfo, 0)
	tx.EachEndpointOfService(svc.Namespace, svc.Name, func(info *localnetv1.EndpointInfo) {
		info = proto.Clone(info).(*localnetv1.EndpointInfo)

		epNodeName := info.NodeName
		if epNodeName == "" {
			epNodeName = info.Topology[hostnameLabel]
		}

		info.Endpoint.Local = epNodeName == nodeName

		if !info.Conditions.Ready {
			return
		}

		infos = append(infos, info)
	})

	internalEndpoints = make([]*localnetv1.EndpointInfo, 0, len(infos))
	externalEndpoints = make([]*localnetv1.EndpointInfo, 0, len(infos))

	// select endpoints for this service

	for _, info := range infos {
		if !(si.Service.InternalTrafficToLocal && !info.Endpoint.Local) {
			internalEndpoints = append(internalEndpoints, info)
		}
		if !(si.Service.ExternalTrafficToLocal && !info.Endpoint.Local) {
			externalEndpoints = append(externalEndpoints, info)
		}
	}

	// TODO handle TopologyAwareHints

	return
}
