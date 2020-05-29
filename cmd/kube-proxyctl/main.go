package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"google.golang.org/grpc"
	"k8s.io/klog"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

var (
	target = flag.String("target", "127.0.0.1:12090", "local API to reach")
	once   = flag.Bool("once", false, "only one fetch loop")
)

func main() {
	flag.Parse()

	conn, err := grpc.Dial(*target, grpc.WithInsecure())

	if err != nil {
		klog.Fatal("failed to connect: ", err)
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer func() {
		ctxCancel()
		conn.Close()

	}()

	// draft of client run
	epc := localnetv1.NewEndpointsClient(conn)

	nextFilter := &localnetv1.NextFilter{}

	for {
		next, err := epc.Next(ctx, &localnetv1.NextFilter{
			InstanceID: nextFilter.InstanceID,
			Rev:        nextFilter.Rev,
		})
		if err != nil {
			klog.Info("next failed: ", err)
			return
		}

		nextItem, err := next.Recv()
		if err != nil {
			klog.Fatal(err)
		}

		nextFilter = nextItem.Next

		for {
			nextItem, err := next.Recv()

			if err == io.EOF {
				break
			} else if err != nil {
				klog.Fatal(err)
			}

			fmt.Fprintln(os.Stdout, nextItem.Endpoints)
		}

		if *once {
			klog.Info("next filter: ", nextFilter)
			break
		}

		fmt.Fprintln(os.Stdout)
	}
}
