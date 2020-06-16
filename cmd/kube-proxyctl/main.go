package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/klog"

	"github.com/mcluseau/kube-proxy2/pkg/client"
)

var (
	once = flag.Bool("once", false, "only one fetch loop")
)

func main() {
	epc := client.New(flag.CommandLine)

	flag.Parse()

	epc.CancelOnSignals()

	for {
		items := epc.Next()

		if items == nil {
			// canceled
			return
		}

		for _, item := range items {
			fmt.Fprintln(os.Stdout, item)
		}

		if *once {
			klog.Infof("to resume this watch, use --instance-id %d --rev %d", epc.InstanceID, epc.Rev)
			return
		}

		fmt.Fprintln(os.Stdout)
	}
}
