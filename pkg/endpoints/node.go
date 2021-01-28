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
	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
	"m.cluseau.fr/kube-proxy2/pkg/proxystore"
)

func ForNode(tx *proxystore.Tx, si *localnetv1.ServiceInfo, nodeName string) (selection []*localnetv1.EndpointInfo) {
	node := tx.GetNode(nodeName)

	var labels map[string]string

	if node == nil || node.Labels == nil {
		labels = map[string]string{}
	} else {
		labels = node.Labels
	}

	topologyKeys := si.TopologyKeys
	if len(topologyKeys) == 0 {
		topologyKeys = []string{"*"}
	}

	svc := si.Service

	infos := make([]*localnetv1.EndpointInfo, 0)
	tx.EachEndpointOfService(svc.Namespace, svc.Name, func(info *localnetv1.EndpointInfo) {
		infos = append(infos, info)
	})

	for _, topoKey := range topologyKeys {
		ref := ""

		if topoKey != "*" {
			ref = labels[topoKey]

			if ref == "" {
				// we do not have that key, skip
				continue
			}
		}

		for _, info := range infos {
			if info.Conditions.Ready && (topoKey == "*" || (info.Topology != nil && info.Topology[topoKey] == ref)) {
				selection = append(selection, info)
			}
		}

		if len(selection) != 0 {
			return
		}
	}

	return
}
