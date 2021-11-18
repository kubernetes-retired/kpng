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

package ipvssink2

import (
	"os"
	"sigs.k8s.io/kpng/client/serviceevents"


	"k8s.io/klog"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
)

func init() {
	backendcmd.Register("to-ipvs2", func() backendcmd.Cmd { return &ipvsBackend{} })
}

type ipvsBackend struct {
	localsink.Config
	dryRun           bool
	nodeAddresses    []string
	schedulingMethod string
	weight           int32
}

var _ decoder.Interface = &ipvsBackend{}

func (s *ipvsBackend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(serviceevents.Wrap(s)))
}

func (s *ipvsBackend) Setup() {
	klog.V(1).Infof("-->ipvsBackend....Setup")
}


// ------------------------------------------------------------------------
// Decoder sink backend interface
//

// Sync signals an stream sync event
func (s *ipvsBackend) Sync() {
	klog.V(1).Infof("-->Sync")
}

// WaitRequest see localsink.Sink#WaitRequest
func (s *ipvsBackend) WaitRequest() (nodeName string, err error) {
	name, _ := os.Hostname(); return name, nil
}

// Reset see localsink.Sink#Reset
func (s *ipvsBackend) Reset() {
	klog.V(1).Infof("-->Reset")
}

func (s *ipvsBackend) SetService(svc *localnetv1.Service) {
	klog.V(1).Infof("-->SetService(%v)", svc)
}

func (s *ipvsBackend) DeleteService(namespace, name string) {
	klog.V(1).Infof("-->DeleteService(%v, %v)", namespace, name)


}

func (s *ipvsBackend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	klog.Infof("-->SetEndpoint(%q, %q, %q, %v)", namespace, serviceName, key, endpoint)

}

func (s *ipvsBackend) DeleteEndpoint(namespace, serviceName, key string) {
	klog.Infof("-->DeleteEndpoint(%q, %q, %q)", namespace, serviceName, key)

}

// ------------------------------------------------------------------------
// (IP, port) listener interface
//

var _ serviceevents.IPPortsListener = &ipvsBackend{}

func (s *ipvsBackend) AddIPPort(svc *localnetv1.Service, ip string, _ serviceevents.IPKind, port *localnetv1.PortMapping) {
	klog.Infof("-->AddIPPort(%v, %v, %v)", svc, ip, port)
}

func (s *ipvsBackend) DeleteIPPort(svc *localnetv1.Service, ip string, _ serviceevents.IPKind, port *localnetv1.PortMapping) {
	klog.Infof("-->DeleteIPPort(%v, %v, %v)", svc, ip, port)
}

// ------------------------------------------------------------------------
// IP listener interface
//

var _ serviceevents.IPsListener = &ipvsBackend{}

func (s *ipvsBackend) AddIP(svc *localnetv1.Service, ip string, ipKind serviceevents.IPKind) {
	klog.Infof("-->AddIP(%v, %v, %v)", svc, ip, ipKind)
}
func (s *ipvsBackend) DeleteIP(svc *localnetv1.Service, ip string, ipKind serviceevents.IPKind) {
	klog.Infof("-->DeleteIP(%v, %v, %v)", svc, ip, ipKind)
}
