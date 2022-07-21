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

package iptables

import (
	"sync"

	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/exec"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/backends/iptables/util"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
	"sigs.k8s.io/kpng/client/localsink/filterreset/pipe"
	"sigs.k8s.io/kpng/client/plugins/conntrack"
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
	return filterreset.New(pipe.New(decoder.New(s), decoder.New(conntrack.NewSink())))
}

func (s *Backend) BindFlags(flags *pflag.FlagSet) {
}

func (s *Backend) Setup() {
	hostname = s.NodeName
	IptablesImpl = make(map[v1.IPFamily]*iptables)
	for _, protocol := range []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol} {
		iptable := NewIptables()
		iptable.iptInterface = util.NewIPTableExec(exec.New(), util.Protocol(protocol))
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
