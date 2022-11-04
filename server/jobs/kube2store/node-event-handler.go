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
	v1 "k8s.io/api/core/v1"

	globalv1 "sigs.k8s.io/kpng/api/globalv1"
	proxystore "sigs.k8s.io/kpng/server/proxystore"
)

const (
	nodeZoneLabel = "topology.kubernetes.io/zone"
)

type nodeEventHandler struct{ eventHandler }

func (h *nodeEventHandler) OnAdd(obj interface{}) {
	node := obj.(*v1.Node)

	// keep only what we want
	n := &globalv1.Node{
		Name: node.Name,
		Topology: &globalv1.TopologyInfo{
			Node: node.Name,
			Zone: node.Labels[nodeZoneLabel],
		},
		Labels:      globsFilter(node.Labels, h.config.NodeLabelGlobs),
		Annotations: globsFilter(node.Annotations, h.config.NodeAnnotationGlobs),
	}

	h.s.Update(func(tx *proxystore.Tx) {
		tx.SetNode(n)

		h.updateSync(proxystore.Nodes, tx)
	})
}

func (h *nodeEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}

func (h *nodeEventHandler) OnDelete(oldObj interface{}) {
	node := oldObj.(*v1.Node)

	h.s.Update(func(tx *proxystore.Tx) {
		tx.DelNode(node.Name)
		h.updateSync(proxystore.Nodes, tx)
	})
}
