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

package nft

import (
	"github.com/spf13/pflag"

	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
	"sigs.k8s.io/kpng/client/localsink/fullstate/fullstatepipe"
	"sigs.k8s.io/kpng/client/plugins/conntrack"
)

type backend struct {
	cfg localsink.Config
}

func init() {
	backendcmd.Register("to-nft", func() backendcmd.Cmd { return &backend{} })
}

func (b *backend) BindFlags(flags *pflag.FlagSet) {
	b.cfg.BindFlags(flags)
	BindFlags(flags)
}

func (b *backend) Sink() localsink.Sink {
	sink := fullstate.New(&b.cfg)

	PreRun()

	ct := conntrack.New()
	sink.Callback = fullstatepipe.New(fullstatepipe.ParallelSendSequenceClose,
		Callback,
		ct.Callback,
	).Callback

	return sink
}
