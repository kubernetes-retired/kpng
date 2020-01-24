package main

import (
	"context"
	"flag"
	"os"

	"github.com/spf13/cobra"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/endpoints"
	"github.com/mcluseau/kube-proxy2/pkg/proxy"
	srvendpoints "github.com/mcluseau/kube-proxy2/pkg/server/endpoints"

	"k8s.io/klog"
)

const (
	testGRPC = false
)

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
