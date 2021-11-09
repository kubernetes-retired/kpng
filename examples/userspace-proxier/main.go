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
