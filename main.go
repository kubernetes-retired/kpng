package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/mcluseau/kube-localnet-api/pkg/api/localnetv1"
	"github.com/mcluseau/kube-localnet-api/pkg/endpoints"
	"github.com/mcluseau/kube-localnet-api/pkg/proxy"
	srvendpoints "github.com/mcluseau/kube-localnet-api/pkg/server/endpoints"
	"github.com/spf13/cobra"

	"k8s.io/klog"
)

func main() {
	proxy.InitFlags(flag.CommandLine)

	cmd := cobra.Command{
		Use: "localnet-api",
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

	go func() {
		proxy.WaitForTermSignal()

		ctxCancel()
		conn.Close()

		srv.Stop()

		// FIXME srv.Stop() doesn't make app exits...
		time.Sleep(10 * time.Millisecond)
		os.Exit(0)
	}()

	go func() {
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
	}()

	select {}
}
