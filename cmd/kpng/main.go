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
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"k8s.io/klog"

	"sigs.k8s.io/kpng/server/pkg/proxy"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	version    = "(unknown)"
)

func main() {
	klog.InitFlags(flag.CommandLine)

	cmd := cobra.Command{
		Use: "kpng",
	}

	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	cmd.AddCommand(
		kube2storeCmd(),
		file2storeCmd(),
		api2storeCmd(),
		local2sinkCmd(),
		versionCmd(),
	)

	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}

func setupGlobal() (ctx context.Context) {
	ctx, cancel := context.WithCancel(context.Background())

	// handle exit signals
	go func() {
		proxy.WaitForTermSignal()
		cancel()

		proxy.WaitForTermSignal()
		klog.Fatal("forced exit after second term signal")
		os.Exit(1)
	}()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			klog.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	return
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print the version and quit",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}
}
