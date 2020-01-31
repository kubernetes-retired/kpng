package endpoints

import v1 "k8s.io/api/core/v1"

type nodesEventHandler eventHandler

func (h nodesEventHandler) onChange() {
	h.c.eventL.Lock()
	defer h.c.eventL.Unlock()

	// recompute all endpoints
	// XXX we may index endpoint dependencies to limit that if scaling becomes an issue
	updated := false
	for _, src := range h.c.sources {
		if h.c.updateEndpoints(src, h.c.nodeLabels) {
			updated = true
		}
	}

	h.c.bumpRev(updated)
}

func (h nodesEventHandler) OnAdd(obj interface{}) {
	node := obj.(*v1.Node)

	eventHandler(h).onChange()
}
func (h nodesEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}
func (h nodesEventHandler) OnDelete(oldObj interface{}) {
	node := oldObj.(*v1.Node)

	eventHandler(h).handle(svc.Namespace, svc.Name, func(src *correlationSource) {
		src.Service = nil
	})
}
