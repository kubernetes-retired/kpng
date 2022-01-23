package main

import (
	"flag"
	"io"
	"log"
	"time"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/klog/v2"
	utilnetsh "k8s.io/kubernetes/pkg/util/netsh"
	"k8s.io/utils/exec"
	netutils "k8s.io/utils/net"
	"sigs.k8s.io/kpng/api/localnetv1"
)

var (
	proxier Provider

	bindAddress        string
	portRange          string
	syncPeriodDuration time.Duration
	udpIdleTimeout     time.Duration
)

type userspaceBackend struct {
	nodeName  string
	services  map[string]*service
	ips       map[string]bool
	listeners map[string]io.Closer
}

func (b *userspaceBackend) BindFlags() {
	flag.StringVar(&bindAddress, "bind-address", "0.0.0.0", "bind address")
	flag.StringVar(&portRange, "port-range", "36000-37000", "port range")
	flag.DurationVar(&syncPeriodDuration, "sync-period-duration", 15*time.Second, "sync period duration")
	flag.DurationVar(&udpIdleTimeout, "udp-idle-timeout", 10*time.Second, "UDP idle timeout")
}

func (b *userspaceBackend) Setup() {
	var err error

	klog.V(0).InfoS("Using Windows userspace Proxier.")

	execer := exec.New()
	var netshInterface utilnetsh.Interface
	netshInterface = utilnetsh.New(execer)
	proxier, err = NewProxier(
		NewLoadBalancerRR(),
		netutils.ParseIPSloppy(bindAddress),
		netshInterface,
		*utilnet.ParsePortRangeOrDie(portRange),
		syncPeriodDuration,
		udpIdleTimeout,
	)

	if err != nil {
		log.Fatal(err)
	}
}

// Sync signals an stream sync event
func (b *userspaceBackend) Sync() {
	proxier.Sync()
}

// WaitRequest see localsink.Sink#WaitRequest
func (b *userspaceBackend) WaitRequest() (nodeName string, err error) {
	return b.nodeName, nil
}

// Reset see localsink.Sink#Reset
func (b *userspaceBackend) Reset() { /* noop */ }

// SetService is called when a service is added or updated
func (b *userspaceBackend) SetService(svc *localnetv1.Service) {
	key := svc.NamespacedName()
	if oldSvc, ok := b.services[key]; ok {
		proxier.OnServiceUpdate(oldSvc.internalSvc, svc)
	} else {
		proxier.OnServiceAdd(svc)
	}
	b.services[key] = &service{Name: key, internalSvc: svc}
}

// DeleteService is called when a service is deleted
func (b *userspaceBackend) DeleteService(namespace, name string) {
	key := namespace + "/" + name
	proxier.OnServiceDelete(b.services[key].internalSvc)
	delete(b.services, key)
}

// SetEndpoint is called when an endpoint is added or updated
func (b *userspaceBackend) SetEndpoint(namespace, serviceName, epKey string, endpoint *localnetv1.Endpoint) {
	svc := b.services[namespace+"/"+serviceName]
	svc.AddEndpoint(epKey, endpoint)
	proxier.OnEndpointsAdd(endpoint, svc.internalSvc)
}

// DeleteEndpoint is called when an endpoint is deleted
func (b *userspaceBackend) DeleteEndpoint(namespace, serviceName, epKey string) {
	key := namespace + "/" + serviceName
	svc := b.services[key]
	if ep := svc.GetEndpoint(epKey); ep.key == epKey {
		proxier.OnEndpointsDelete(ep.internalEp, svc.internalSvc)
	}
	svc.DeleteEndpoint(epKey)
}
