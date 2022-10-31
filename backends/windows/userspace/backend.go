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

package userspace

import (
	"io"
	"log"
	"time"

	"github.com/spf13/pflag"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	netutils "k8s.io/utils/net"

	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
)

func init() {
	backendcmd.Register("to-winuserspace", func() backendcmd.Cmd { return &userspaceBackend{} })
}

var (
	proxier Provider

	bindAddress        string
	portRange          string
	syncPeriodDuration time.Duration
	udpIdleTimeout     time.Duration
)

type userspaceBackend struct {
	localsink.Config

	services  map[string]*service
	ips       map[string]bool
	listeners map[string]io.Closer
}

var _ decoder.Interface = &userspaceBackend{}

func (b *userspaceBackend) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&bindAddress, "bind-address", "0.0.0.0", "bind address")
	flags.StringVar(&portRange, "port-range", "36000-37000", "port range")
	flags.DurationVar(&syncPeriodDuration, "sync-period-duration", 15*time.Second, "sync period duration")
	flags.DurationVar(&udpIdleTimeout, "udp-idle-timeout", 10*time.Second, "UDP idle timeout")
}

func (b *userspaceBackend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(b))
}

func (b *userspaceBackend) Setup() {
	var err error

	klog.V(0).InfoS("Using Windows Userspace Proxier. (this is a deprecated mode).")

	execer := exec.New()
	var netshInterface Interface
	netshInterface = New(execer)
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
func (b *userspaceBackend) WaitRequest() (NodeName string, err error) {
	return b.NodeName, nil
}

// Reset see localsink.Sink#Reset
func (b *userspaceBackend) Reset() { /* noop */ }

// SetService is called when a service is added or updated
func (b *userspaceBackend) SetService(svc *localv1.Service) {
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
func (b *userspaceBackend) SetEndpoint(namespace, serviceName, epKey string, endpoint *localv1.Endpoint) {
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
