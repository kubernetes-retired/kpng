package endpoints

import (
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1beta1"
	"k8s.io/client-go/tools/cache"
)

type eventHandler struct {
	c        *Correlator
	synced   *bool
	informer cache.SharedIndexInformer
}

func (h eventHandler) handle(namespace, name string, handle func(*correlationSource)) {
	h.c.eventL.Lock()
	defer h.c.eventL.Unlock()

	// get the current source
	key := namespace + "/" + name
	src := h.c.sources[key]

	handle(&src)

	// update the source
	if src.IsEmpty() {
		delete(h.c.sources, key)
	} else {
		h.c.sources[key] = src
	}

	*h.synced = h.informer.HasSynced()

	updated := h.c.updateEndpoints(src)

	h.c.bumpRev(updated)
}

type serviceEventHandler eventHandler

func (h serviceEventHandler) OnAdd(obj interface{}) {
	svc := obj.(*v1.Service)

	eventHandler(h).handle(svc.Namespace, svc.Name, func(src *correlationSource) {
		src.Service = svc
	})
}
func (h serviceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}
func (h serviceEventHandler) OnDelete(oldObj interface{}) {
	svc := oldObj.(*v1.Service)

	eventHandler(h).handle(svc.Namespace, svc.Name, func(src *correlationSource) {
		src.Service = nil
	})
}

type endpointsEventHandler eventHandler

func (h endpointsEventHandler) OnAdd(obj interface{}) {
	eps := obj.(*v1.Endpoints)

	eventHandler(h).handle(eps.Namespace, eps.Name, func(src *correlationSource) {
		src.Endpoints = eps
	})
}
func (h endpointsEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}
func (h endpointsEventHandler) OnDelete(oldObj interface{}) {
	eps := oldObj.(*v1.Endpoints)

	eventHandler(h).handle(eps.Namespace, eps.Name, func(src *correlationSource) {
		src.Endpoints = nil
	})
}

type sliceEventHandler eventHandler

func (h sliceEventHandler) nameFrom(eps *discovery.EndpointSlice) string {
	if eps.Labels == nil {
		return ""
	}
	return eps.Labels[discovery.LabelServiceName]
}

func (h sliceEventHandler) OnAdd(obj interface{}) {
	eps := obj.(*discovery.EndpointSlice)

	name := h.nameFrom(eps)
	if name == "" {
		// no name => not associated with a service => ignore
		return
	}

	eventHandler(h).handle(eps.Namespace, name, func(src *correlationSource) {
		newSlices := make([]*discovery.EndpointSlice, len(src.Slices), len(src.Slices)+1)

		set := false
		for idx, s := range src.Slices {
			if s.Name == eps.Name {
				newSlices[idx] = eps
				set = true
			} else {
				newSlices[idx] = s
			}
		}

		if !set {
			newSlices = append(newSlices, eps)
			// TODO later: sort, or use Set
		}

		src.Slices = newSlices
	})
}
func (h sliceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	oldName := h.nameFrom(oldObj.(*discovery.EndpointSlice))
	newName := h.nameFrom(newObj.(*discovery.EndpointSlice))

	if oldName != "" && oldName != newName {
		// delete previous if service-name label has changed
		h.OnDelete(oldObj)
	}

	// same as adding
	h.OnAdd(newObj)
}
func (h sliceEventHandler) OnDelete(oldObj interface{}) {
	eps := oldObj.(*discovery.EndpointSlice)

	name := h.nameFrom(eps)
	if name == "" {
		// no name => was not recorded => nothing to do on delete
		return
	}

	eventHandler(h).handle(eps.Namespace, name, func(src *correlationSource) {
		newSlices := make([]*discovery.EndpointSlice, 0, len(src.Slices))

		for idx, s := range src.Slices {
			if s.Name == eps.Name {
				continue
			}
			newSlices[idx] = s
		}

		src.Slices = newSlices
	})
}
