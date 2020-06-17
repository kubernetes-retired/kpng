package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"runtime/pprof"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"k8s.io/klog"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/proxy"
	srvendpoints "github.com/mcluseau/kube-proxy2/pkg/server/endpoints"
)

const (
	testGRPC = false
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	bindSpec   = flag.String("listen", "tcp://127.0.0.1:12090", "local API listen spec formatted as protocol://address")
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

	// setup correlator
	srvendpoints.Setup(srv)

	// handle exit signals
	go func() {
		proxy.WaitForTermSignal()
		srv.Stop()
	}()

	if *bindSpec != "" {
		parts := strings.SplitN(*bindSpec, "://", 2)
		if len(parts) != 2 {
			klog.Error("invalid listen spec: expected protocol://address format but got ", *bindSpec)
			os.Exit(1)
		}

		protocol, addr := parts[0], parts[1]

		// handle protocol specifics
		afterListen := func() {}
		switch protocol {
		case "unix":
			os.Remove(addr)
			prevMask := syscall.Umask(0007)
			afterListen = func() { syscall.Umask(prevMask) }
		}

		lis, err := net.Listen(protocol, addr)
		if err != nil {
			klog.Error("failed to listen on ", *bindSpec, ": ", err)
			os.Exit(1)
		}

		afterListen()

		klog.Info("API listening on ", *bindSpec)
		go klog.Fatal(srv.GRPC.Serve(lis))
	}

	if testGRPC {
		doTestGRPC(srv)
	}

	// wait and exit
	_, _ = <-srv.QuitCh
}

func doTestGRPC(srv *proxy.Server) {
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
