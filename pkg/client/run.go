package client

import (
	"flag"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"k8s.io/klog"
)

type HandlerFunc func(items []*localnetv1.ServiceEndpoints)

// Run the client with the standard options
func Run(handlers ...HandlerFunc) {
	once := flag.Bool("once", false, "only one fetch loop")

	epc := New(flag.CommandLine)

	flag.Parse()

	epc.CancelOnSignals()

	for {
		items, canceled := epc.Next()

		if canceled {
			return
		}

		for _, handler := range handlers {
			handler(items)
		}

		if *once {
			klog.Infof("to resume this watch, use --instance-id %d --rev %d", epc.InstanceID, epc.Rev)
			return
		}
	}
}
