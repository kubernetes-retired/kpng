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

package serviceevents

import (
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/localsink/decoder"
)

type wrapper struct {
	// backend
	decoder.Interface
	// listener
	l *ServicesListener
}

var _ decoder.Interface = wrapper{}

// Wrap a decoder so it receives detailled events depending on which interfaces
// it implements.
//
// A good practice is to ensure your decoder is implementing what you expect
// this way:
//
//     type MyBackend struct { }
//
//     var _ servicevents.PortsListener   = &MyBackend{}
//     var _ servicevents.IPsListener     = &MyBackend{}
//     var _ servicevents.IPPortsListener = &MyBackend{}
//
func Wrap(backend decoder.Interface) decoder.Interface {
	l := New()

	if v, ok := backend.(PortsListener); ok {
		l.PortsListener = v
	}
	if v, ok := backend.(IPsListener); ok {
		l.IPsListener = v
	}
	if v, ok := backend.(IPPortsListener); ok {
		l.IPPortsListener = v
	}
	if v, ok := backend.(SessionAffinityListener); ok {
		l.SessionAffinityListener = v
	}
	if v, ok := backend.(TrafficPolicyListener); ok {
		l.TrafficPolicyListener = v
	}

	wrap := wrapper{
		Interface: backend,
		l:         l,
	}
	return wrap
}

func (w wrapper) SetService(service *localv1.Service) {
	w.Interface.SetService(service)
	w.l.SetService(service)
}

func (w wrapper) DeleteService(namespace, name string) {
	w.l.DeleteService(namespace, name)
	w.Interface.DeleteService(namespace, name)
}
