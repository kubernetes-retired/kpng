package endpoints

import (
	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/proxystore"
)

func ForNode(tx *proxystore.Tx, svc *localnetv1.Service, topologyKeys []string, nodeName string) (selection []*localnetv1.Endpoint) {
	node := tx.GetNode(nodeName)

	var labels map[string]string

	if node == nil || node.Labels == nil {
		labels = map[string]string{}
	} else {
		labels = node.Labels
	}

	if len(topologyKeys) == 0 {
		topologyKeys = []string{"*"}
	}

	for _, topoKey := range topologyKeys {
		ref := ""

		if topoKey != "*" {
			ref = labels[topoKey]

			if ref == "" {
				// we do not have that key, skip
				continue
			}
		}

		tx.EachEndpointOfService(svc.Namespace, svc.Name, func(info *localnetv1.EndpointInfo) {
			if info.Conditions.Ready && (topoKey == "*" || (info.Topology != nil && info.Topology[topoKey] == ref)) {
				selection = append(selection, info.Endpoint)
			}
		})

		if len(selection) != 0 {
			return
		}
	}

	return
}
