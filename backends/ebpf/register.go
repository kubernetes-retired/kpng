/*
Copyright 2022 The Kubernetes Authors.

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

package ebpf

import (
	"github.com/spf13/pflag"

	"k8s.io/klog"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
	"sigs.k8s.io/kpng/client/localsink/fullstate/fullstatepipe"
)

var ebc ebpfController

type backend struct {
	cfg localsink.Config
}

func init() {
	backendcmd.Register("to-ebpf", func() backendcmd.Cmd { return &backend{} })
}

func (s *backend) BindFlags(flags *pflag.FlagSet) {
}

func (s *backend) Reset() { /* noop */ }

// WaitRequest see localsink.Sink#WaitRequest
// func (s *backend) WaitRequest() (nodeName string, err error) {
// 	name, _ := os.Hostname()
// 	return name, nil
// }

func (s *backend) Setup() {
	ebc = ebpfSetup()
	klog.Infof("Loading ebpf maps and program %+v", ebc)
}

func (b *backend) Sync() { /* no-op */ }

func (b *backend) Sink() localsink.Sink {
	sink := fullstate.New(&b.cfg)

	sink.Callback = fullstatepipe.New(fullstatepipe.ParallelSendSequenceClose,
		ebc.Callback,
	).Callback

	sink.SetupFunc = b.Setup

	return sink
}
