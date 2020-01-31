package main

import (
	"context"
	"flag"
	"log"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"k8s.io/klog"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/endpoints"
	"github.com/mcluseau/kube-proxy2/pkg/proxy"
	srvendpoints "github.com/mcluseau/kube-proxy2/pkg/server/endpoints"
)

const (
	testGRPC = false
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	proxy.InitFlags(flag.CommandLine)

	cmd := cobra.Command{
		Use: "proxy",
		Run: run,
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}

func run(_ *cobra.Command, _ []string) {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	srv, err := proxy.NewServer()

	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	endpointsCorrelator := endpoints.NewCorrelator(srv)
	go endpointsCorrelator.Run(srv.QuitCh)

	// handle exit signals
	go func() {
		proxy.WaitForTermSignal()
		srv.Stop()
	}()

	if testGRPC {
		doTestGRPC(srv, endpointsCorrelator)
	}

	// wait and exit
	_, _ = <-srv.QuitCh
}

func doTestGRPC(srv *proxy.Server, endpointsCorrelator *endpoints.Correlator) {
	// setup gRPC
	localnetv1.RegisterEndpointsServer(srv.GRPC, &srvendpoints.Server{
		InstanceID: srv.InstanceID,
		Correlator: endpointsCorrelator,
	})

	conn, err := srv.InProcessClient(func(err error) { klog.Error("serve failed: ", err) })
	if err != nil {
		klog.Error("error setting up in-memory gRPC: ", err)
		os.Exit(1)
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer func() {
		ctxCancel()
		conn.Close()

	}()

	// draft of client run
	epc := localnetv1.NewEndpointsClient(conn)

	klog.Info("next: calling...")
	next, err := epc.Next(ctx, &localnetv1.NextFilter{})
	if err != nil {
		klog.Info("next failed: ", err)
		return
	}

	klog.Info("next: success")

	for {
		nextItem, err := next.Recv()
		if err != nil {
			klog.Info("next: error: ", err)
			break
		}
		klog.Info("next: - item: ", nextItem)
	}
}
