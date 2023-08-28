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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kpng/server/pkg/metrics"

	// import existent backends quietly
	_ "sigs.k8s.io/kpng/cmd/kpng/storecmds"

	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/server/pkg/proxy"
)

var (
	cpuprofile    = flag.String("cpuprofile", "", "write cpu profile to file")
	exportMetrics = flag.String("exportMetrics", "", "start metrics server on the specified IP:PORT")
)

// main starts the kpng program by running the command sent by the user.  This is the entry point to kpng!
func main() {
	metricsPort := "9099"
	setupPrometheusServer(context.TODO(), fmt.Sprintf("0.0.0.0:%s", metricsPort))

	klog.InitFlags(flag.CommandLine)

	cmd := cobra.Command{
		Use: "kpng",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			klog.Infof("persistent pre run: %s\n", cmd.Name())
		},
	}

	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	cmd.AddCommand(
		kube2storeCmd(), // no-op?
		file2storeCmd(),
		api2storeCmd(),
		local2sinkCmd(),
		versionCmd(),
	)

	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}

func setupPrometheusServer(ctx context.Context, address string) {
	prometheus.MustRegister(metrics.Kpng_k8s_api_events)
	prometheus.MustRegister(metrics.Kpng_node_local_events)
	klog.Infof("exporting metrics to: %v ", address)
	metrics.StartMetricsServer(address, ctx.Done())
}

// setupGlobal sets up global processes that need to run regardless of what mode you are running KPNG in.
// this is a grab bag where you put stuff that, one way or other, has to happen.

func setupGlobal() (ctx context.Context) {
	ctx, cancel := context.WithCancel(context.Background())

	out := ""
	if exportMetrics != nil {
		out = *exportMetrics
	}
	klog.Infof("setup global, exportMetrics value: %+v\n", out)

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
