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
			klog.Infof("finished")
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

// RunWithIterator runs the client with the standard options, using the iterated version of Next.
// It should consume less memory as the dataset is processed as it's read instead of buffered.
// The handler MUST check iter.Err to ensure the dataset was fuly retrieved without error.
func RunWithIterator(handler func(*Iterator)) {
	once := flag.Bool("once", false, "only one fetch loop")

	epc := New(flag.CommandLine)

	flag.Parse()

	epc.CancelOnSignals()

	for {
		iter := epc.NextIterator()

		if iter.Canceled {
			klog.Infof("finished")
			return
		}

		handler(iter)

		if iter.RecvErr != nil {
			klog.Error("recv error: ", iter.RecvErr)
		}

		if *once {
			klog.Infof("to resume this watch, use --instance-id %d --rev %d", epc.InstanceID, epc.Rev)
			return
		}
	}
}
