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
	"context"
	"flag"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"k8s.io/klog"

	"m.cluseau.fr/kpng/jobs/kube2store"
	"m.cluseau.fr/kpng/pkg/proxy"
	"m.cluseau.fr/kpng/pkg/server"
	srvendpoints "m.cluseau.fr/kpng/pkg/server/endpoints"
	srvglobal "m.cluseau.fr/kpng/pkg/server/global"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	bindSpec   = flag.String("listen", "tcp://127.0.0.1:12090", "local API listen spec formatted as protocol://address")

	k2sCfg = &kube2store.Config{}
)

func main() {
	proxy.InitFlags(flag.CommandLine)

	cmd := cobra.Command{
		Use: "proxy",
		Run: run,
	}

	k2sCfg.BindFlags(cmd.Flags())

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}

func run(_ *cobra.Command, _ []string) {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			klog.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	srv, err := proxy.NewServer()

	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// start kube2store
	kube2store.Job{
		Kube:   srv.Client,
		Store:  srv.Store,
		Config: k2sCfg,
	}.Run(ctx)

	// setup server
	srvendpoints.Setup(srv.GRPC, srv.Store)
	srvglobal.Setup(srv.GRPC, srv.Store)

	// handle exit signals
	go func() {
		proxy.WaitForTermSignal()
		srv.Stop()
		cancel()
	}()

	if *bindSpec != "" {
		lis := server.MustListen(*bindSpec)
		go func() {
			err := srv.GRPC.Serve(lis)
			if err != nil {
				klog.Fatal(err)
			}
		}()
	}

	// wait and exit
	_, _ = <-srv.QuitCh
}
