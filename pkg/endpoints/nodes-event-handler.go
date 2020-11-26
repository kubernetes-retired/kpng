package endpoints

import (
	"flag"

	v1 "k8s.io/api/core/v1"

	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
	"m.cluseau.fr/kube-proxy2/pkg/proxy"
	"m.cluseau.fr/kube-proxy2/pkg/proxystore"
)

type nodesEventHandler struct{ eventHandler }

var myNodeName = flag.String("node-name", "", "Node name override")

func (h nodesEventHandler) OnAdd(obj interface{}) {
	node := obj.(*v1.Node)

	// remove some fat
	n := &localnetv1.Node{
		Name:   node.Name,
		Labels: node.Labels,
	}

	h.s.Update(func(tx *proxystore.Tx) {
		tx.SetNode(n)

		if !proxy.ManageEndpointSlices {
			// endpoints => need to update all matching topologies
			tx.Each(proxystore.Endpoints, func(kv *proxystore.KV) bool {
				if kv.Endpoint.NodeName == n.Name {
					kv.Endpoint.Topology = n.Labels
				}
				return true
			})
		}

		h.updateSync(proxystore.Nodes, tx)
	})
}

func (h nodesEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}

func (h nodesEventHandler) OnDelete(oldObj interface{}) {
	node := oldObj.(*v1.Node)

	h.s.Update(func(tx *proxystore.Tx) {
		tx.DelNode(node.Name)
		h.updateSync(proxystore.Nodes, tx)
	})
}
