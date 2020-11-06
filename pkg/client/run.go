package client

import (
	"flag"
	"os"
	"runtime/pprof"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"k8s.io/klog"
)

type HandlerFunc func(items []*localnetv1.ServiceEndpoints)

func Default() (*EndpointsClient, bool) {
	once := flag.Bool("once", false, "only one fetch loop")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")

	epc := New(flag.CommandLine)

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			klog.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	epc.CancelOnSignals()

	return epc, *once
}

// Run the client with the standard options
func Run(handlers ...HandlerFunc) {
	epc, once := Default()

	for {
		items, canceled := epc.Next()

		if canceled {
			klog.Infof("finished")
			return
		}

		for _, handler := range handlers {
			handler(items)
		}

		if once {
			klog.Infof("to resume this watch, use --instance-id %d --rev %d", epc.InstanceID, epc.Rev)
			return
		}
	}
}

// RunWithIterator runs the client with the standard options, using the iterated version of Next.
// It should consume less memory as the dataset is processed as it's read instead of buffered.
// The handler MUST check iter.Err to ensure the dataset was fuly retrieved without error.
func RunWithIterator(handler func(*Iterator)) {
	epc, once := Default()

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

		if once {
			klog.Infof("to resume this watch, use --instance-id %d --rev %d", epc.InstanceID, epc.Rev)
			return
		}
	}
}
