package iptables

import (
	"sync"

	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/exec"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
)

type Backend struct {
	localsink.Config
}

var wg = sync.WaitGroup{}
var IptablesImpl map[v1.IPFamily]*iptables
var hostname string
var _ decoder.Interface = &Backend{}

func New() *Backend {
	return &Backend{}
}

func (s *Backend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(s))
}

func (s *Backend) BindFlags(flags *pflag.FlagSet) {
}

func (s *Backend) Setup() {
	hostname = s.NodeName
	IptablesImpl = make(map[v1.IPFamily]*iptables)
	for _, protocol := range []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol} {
		iptable := NewIptables()
		iptable.iptInterface = NewIPTableExec(exec.New(), Protocol(protocol))
		iptable.serviceChanges = NewServiceChangeTracker(newServiceInfo, protocol, iptable.recorder)
		iptable.endpointsChanges = NewEndpointChangeTracker(hostname, protocol, iptable.recorder)
		IptablesImpl[protocol] = iptable
	}
}

func (s *Backend) Reset() { /* noop, we're wrapped in filterreset */ }

func (s *Backend) Sync() {
	for _, impl := range IptablesImpl {
		wg.Add(1)
		go impl.sync()
	}
	wg.Wait()
}

func (s *Backend) SetService(svc *localnetv1.Service) {
	for _, impl := range IptablesImpl {
		impl.serviceChanges.Update(svc)
	}
}

func (s *Backend) DeleteService(namespace, name string) {
	for _, impl := range IptablesImpl {
		impl.serviceChanges.Delete(namespace, name)
	}
}

func (s *Backend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	for _, impl := range IptablesImpl {
		impl.endpointsChanges.EndpointUpdate(namespace, serviceName, key, endpoint)
	}

}

func (s *Backend) DeleteEndpoint(namespace, serviceName, key string) {
	for _, impl := range IptablesImpl {
		impl.endpointsChanges.EndpointUpdate(namespace, serviceName, key, nil)
	}
}
