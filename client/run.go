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

package client

import (
	"flag"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	// "k8s.io/klog"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

type HandleFunc func(items []*ServiceEndpoints)
type HandleChFunc = fullstate.Callback

func Run(handler HandleFunc, extraBindFlags ...func(*pflag.FlagSet)) {
	RunCh(ArrayBackend(handler), extraBindFlags...)
}

func RunCh(backend fullstate.Callback, extraBindFlags ...func(*pflag.FlagSet)) {
	r := &Runner{}

	cmd := &cobra.Command{
		Run: func(_ *cobra.Command, _ []string) { r.RunBackend(backend) },
	}

	flags := cmd.Flags()
	r.BindFlags(flags)

	for _, bindFlags := range extraBindFlags {
		bindFlags(flags)
	}

	cmd.Execute()
}

type Runner struct {
	once       bool
	cpuprofile string
	NodeName   string

	epc *EndpointsClient
}

func (r *Runner) BindFlags(flags FlagSet) {
	flag.BoolVar(&r.once, "once", false, "only one fetch loop")
	flag.StringVar(&r.cpuprofile, "cpuprofile", "", "write cpu profile to file")
	flag.StringVar(&r.NodeName, "node-name", func() string { s, _ := os.Hostname(); return s }(), "node name to request to the proxy server")

	r.epc = New(flags)
}

// ArrayBackend creates a Callback from the given array handlers
func ArrayBackend(handlers ...HandleFunc) fullstate.Callback {
	return fullstate.ArrayCallback(func(items []*fullstate.ServiceEndpoints) {
		for _, handler := range handlers {
			handler(items)
		}
	})
}

// RunBackend runs the client with the standard options, using the channeled backend.
// It should consume less memory as the dataset is processed as it's read instead of buffered.
func (r *Runner) RunBackend(handler fullstate.Callback) {
	sink := fullstate.New(&localsink.Config{NodeName: r.NodeName})
	sink.Callback = handler

	r.RunSink(sink)
}

func (r *Runner) RunSink(sink localsink.Sink) {
	r.epc.Sink = sink

	if r.cpuprofile != "" {
		f, err := os.Create(r.cpuprofile)
		if err != nil {
			//klog.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	r.epc.CancelOnSignals()

	r.epc.Sink.Setup()

	for {
		canceled := r.epc.Next()

		if canceled {
			//klog.Infof("finished")
			return
		}

		if r.once {
			return
		}
	}
}
