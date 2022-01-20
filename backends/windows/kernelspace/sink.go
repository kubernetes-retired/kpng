//go:build windows
// +build windows

/*
Copyright 2017-2022 The Kubernetes Authors.

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

package winkernel

import (
	"os"

	 "k8s.io/klog/v2"

	"sigs.k8s.io/kpng/client/serviceevents"
	"sigs.k8s.io/kpng/client/backendcmd"
        "sigs.k8s.io/kpng/api/localnetv1"
        "sigs.k8s.io/kpng/client/localsink"
        "sigs.k8s.io/kpng/client/localsink/decoder"
        "sigs.k8s.io/kpng/client/localsink/filterreset"
)

var _ decoder.Interface = &Backend{}

type Backend struct {
        localsink.Config
}

func init() {
        backendcmd.Register(
		"to-winkernel",
		func() backendcmd.Cmd { return New() })
}

func New() *Backend {
        return &Backend{}
}

func (s *Backend) Sink() localsink.Sink {
        return filterreset.New(decoder.New(serviceevents.Wrap(s)))
}

func (s *Backend) DeleteEndpoint(namespace, serviceName, key string) {
}
func (s *Backend) SetService(svc *localnetv1.Service) {}

func (s *Backend) DeleteService(namespace, name string) {}

func (s *Backend) Reset() { /* noop */ }

func (s *Backend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint){}

func (s *Backend) Setup() {
	//var err error

	klog.V(0).InfoS("Using Windows Kernel Proxier.")

}

func (s *Backend) Sync() {
    //proxier.Sync()
}

func (s *Backend) WaitRequest() (nodeName string, err error) {
        name, _ := os.Hostname()
        return name, nil
}
