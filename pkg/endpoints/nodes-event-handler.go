package endpoints

import (
	"flag"

	v1 "k8s.io/api/core/v1"
)

type nodesEventHandler eventHandler

var myNodeName = flag.String("node-name", "", "Node name override")

func (h nodesEventHandler) onChange(update func()) {
	h.c.eventL.Lock()
	defer h.c.eventL.Unlock()

	update()

	// recompute all endpoints
	// XXX we may index endpoint dependencies to limit that if scaling becomes an issue
	updated := false
	for _, src := range h.c.sources {
		if h.c.updateEndpoints(src) {
			updated = true
		}
	}

	h.c.bumpRev(updated)
}

func (h nodesEventHandler) OnAdd(obj interface{}) {
	node := obj.(*v1.Node)

	h.onChange(func() {
		h.c.nodesInfo[node.Name] = NodeInfo{
			Labels: node.Labels,
		}
	})
}
func (h nodesEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}

func (h nodesEventHandler) OnDelete(oldObj interface{}) {
	node := oldObj.(*v1.Node)

	h.onChange(func() {
		delete(h.c.nodesInfo, node.Name)
	})
}
