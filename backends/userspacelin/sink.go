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

package userspacelin

import (
	"io"
	"log"
	"time"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	klog "k8s.io/klog/v2"
	netutils "k8s.io/utils/net"

	"k8s.io/utils/exec"

	iptablesutil "sigs.k8s.io/kpng/backends/iptables/util"

	"sync"

	"github.com/spf13/pflag"

	localv1 "sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
)

type Backend struct {
	localsink.Config

	services  map[string]*service
	ips       map[string]bool
	listeners map[string]io.Closer
}

var wg = sync.WaitGroup{}
var proxier *UserspaceLinux

// var usImpl map[v1.IPFamily]*UserspaceLinux
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
	var err error
	// hostname = s.NodeName
	// make a proxier for ipv4
	klog.V(0).InfoS("Using Userspace Proxier!")
	execer := exec.New()
	iptables := iptablesutil.New(execer, iptablesutil.Protocol("IPv4"))
	proxier, err = NewUserspaceLinux(
		NewLoadBalancerRR(),
		netutils.ParseIPSloppy("0.0.0.0"),
		iptables,
		execer,
		utilnet.PortRange{Base: 30000, Size: 2768},
		time.Duration(15),
		time.Duration(15),
		time.Millisecond,
	)
	if err != nil {
		log.Fatal("unable to create proxier: ", err)
	}
}

func (s *Backend) Reset() { /* noop, we're wrapped in filterreset */ }

func (s *Backend) Sync() {
	proxier.syncProxyRules()
}

func (s *Backend) SetService(svc *localv1.Service) {
	key := svc.NamespacedName()
	if s.services == nil {
		s.services = make(map[string]*service)
	}

	if oldSvc, ok := s.services[key]; ok {
		proxier.OnServiceUpdate(oldSvc.internalSvc, svc)
	} else {
		proxier.OnServiceAdd(svc)
	}
	s.services[key] = &service{Name: key, internalSvc: svc}

}

func (s *Backend) DeleteService(namespace, name string) {
	key := namespace + "/" + name
	proxier.OnServiceDelete(s.services[key].internalSvc)
	delete(s.services, key)
}

// name of the endpoint is the same as the service name
func (s *Backend) SetEndpoint(namespace, serviceName, epKey string, endpoint *localv1.Endpoint) {
	svc := s.services[namespace+"/"+serviceName]
	svc.AddEndpoint(epKey, endpoint)
	proxier.OnEndpointsAdd(endpoint, svc.internalSvc)
}

func (s *Backend) DeleteEndpoint(namespace, serviceName, epKey string) {
	key := namespace + "/" + serviceName
	svc := s.services[key]
	if ep := svc.GetEndpoint(epKey); ep.key == epKey {
		proxier.OnEndpointsDelete(ep.internalEp, svc.internalSvc)
	}
}

// 1
// 2 <-- last connection sent here
// 3
// -------------------- rr.OnEndpointsUpdate (1,2,3) (1,2,3,4)
// 1
// 2
// 3 <-- next connection will send here
// 4
// --------------------- rr.OnEndpointsUpdate (1,2,3) (1,2,4)
// 1
// 2
// 4 <-- next connection will send here
// ---------------------
// 1 <-- next connection will send here
// 2
// 4
