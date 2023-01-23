/*
Copyright 2023 The Kubernetes Authors.

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

package ipvsfullsate

import (
	"github.com/spf13/pflag"
	"k8s.io/klog"
	"os"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
	"sigs.k8s.io/kpng/client/localsink/fullstate/fullstatepipe"
	"sigs.k8s.io/kpng/client/plugins/conntrack"
)

var controller Controller

type backend struct {
	cfg localsink.Config
}

func init() {
	// registering the backend with the client
	hostname, err := os.Hostname()
	if err != nil {
		klog.Fatal("Unable to retrieve os hostname")
	}

	klog.V(3).Infof("Registering host %s", hostname)
	backendcmd.Register("to-ipvsfullstate", func() backendcmd.Cmd {
		return &backend{
			cfg: localsink.Config{NodeName: hostname},
		}
	})
}

func (b *backend) BindFlags(flags *pflag.FlagSet) {
	b.cfg.BindFlags(flags)
	BindFlags(flags)
}

func (b *backend) Reset() { /* noop */ }

func (b *backend) Sync() { /* no-op */ }

func (b *backend) Sink() localsink.Sink {
	sink := fullstate.New(&b.cfg)

	// client will invoke Setup()
	sink.SetupFunc = b.Setup

	ct := conntrack.New()

	sink.Callback = fullstatepipe.New(fullstatepipe.ParallelSendSequenceClose,
		controller.Callback,
		ct.Callback,
	).Callback

	return sink
}
