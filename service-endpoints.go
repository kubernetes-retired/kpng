package main

import (
	"bytes"
	"net"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

func processServiceEndpointsChanges(changes <-chan serviceEndpointsChange) {
	for change := range changes {
		klog.V(4).Infof("SE change [s:%v] on %s/%s", change.Synced, change.Namespace, change.Name)
		//updateEndpointsIPs(change)
		//updateServicePortTargets(change)
		updateLocalnetAPI(change)
	}
}

type serviceEndpointsCorrelator struct {
	SvcStore cache.Store
	EPStore  cache.Store

	Synced func() bool

	Changes chan<- serviceEndpointsChange

	l sync.Mutex
}

var _ cache.ResourceEventHandler = &serviceEndpointsCorrelator{}

type serviceEndpointsChange struct {
	Synced    bool
	Namespace string
	Name      string
	Service   *corev1.Service
	Endpoints *corev1.Endpoints
}

func (c *serviceEndpointsChange) Prefix() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, len(c.Namespace)+len(c.Name)+1))
	buf.WriteString(c.Namespace)
	buf.WriteByte('/')
	buf.WriteString(c.Name)
	return buf.Bytes()
}

func (c *serviceEndpointsChange) FullyDefined() bool {
	return c.Service != nil && c.Endpoints != nil
}

type ipDef struct {
	IP       net.IP
	External bool
}

func (c *serviceEndpointsChange) ServiceIPs() (defs []ipDef) {
	if c.Service == nil {
		return
	}

	spec := c.Service.Spec

	defs = make([]ipDef, 0, 1+len(spec.ExternalIPs))

	if spec.ClusterIP == "None" {
		defs = append(defs, ipDef{nil, false})
	} else {
		cIP := net.ParseIP(spec.ClusterIP)
		if cIP != nil {
			defs = append(defs, ipDef{cIP, false})
		}
	}

	for _, extIP := range spec.ExternalIPs {
		ip := net.ParseIP(extIP)
		if ip == nil {
			// TODO log?
			continue
		}
		defs = append(defs, ipDef{ip, true})
	}

	return
}

func (h *serviceEndpointsCorrelator) onChange(obj interface{}) {
	v, ok := obj.(metav1.Object)
	if !ok {
		// not an object? ignore anyway
		return
	}

	ns := v.GetNamespace()
	name := v.GetName()

	h.l.Lock()
	defer h.l.Unlock()

	change := serviceEndpointsChange{
		Namespace: ns,
		Name:      name,
		Synced:    h.Synced(),
	}

	key := ns + "/" + name

	if svc, exists, err := h.SvcStore.GetByKey(key); err != nil {
		panic(err)
	} else if exists {
		change.Service = svc.(*corev1.Service)
	}

	if ep, exists, err := h.EPStore.GetByKey(key); err != nil {
		panic(err)
	} else if exists {
		change.Endpoints = ep.(*corev1.Endpoints)
	}

	h.Changes <- change
}

func (h *serviceEndpointsCorrelator) OnAdd(obj interface{}) {
	h.onChange(obj)
}

func (h *serviceEndpointsCorrelator) OnUpdate(_, newObj interface{}) {
	h.onChange(newObj)
}

func (h *serviceEndpointsCorrelator) OnDelete(obj interface{}) {
	h.onChange(obj)
}
