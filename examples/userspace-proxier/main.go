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

package main

import (
	"flag"
	"io"

	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
	"sigs.k8s.io/kpng/client/serviceevents"
)

func main() {
	// prepare the runner
	runner := client.Runner{}
	runner.BindFlags(flag.CommandLine)

	// prepare the backend => see sink.go
	backend := &userspaceBackend{
		services:  map[string]*service{},
		ips:       map[string]bool{},
		listeners: map[string]io.Closer{},
	}
	backend.BindFlags()

	// parse command line flags
	flag.Parse()

	// setup the backend
	backend.nodeName = runner.NodeName

	// and run!
	runner.RunSink(filterreset.New(decoder.New(serviceevents.Wrap(backend))))
}
