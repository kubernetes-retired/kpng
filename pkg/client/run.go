package client

import (
	"flag"
	"os"
	"runtime/pprof"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"k8s.io/klog"
)

type HandlerFunc func(items []*localnetv1.ServiceEndpoints)

func Default() (epc *EndpointsClient, once bool, stop func()) {
	onceFlag := flag.Bool("once", false, "only one fetch loop")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")

	epc = New(flag.CommandLine)

	flag.Parse()

	once = *onceFlag

	if *cpuprofile == "" {
		stop = func() {}
	} else {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			klog.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		stop = pprof.StopCPUProfile
	}

	epc.CancelOnSignals()

	return
}

// Run the client with the standard options
func Run(filter *localnetv1.EndpointConditions, handlers ...HandlerFunc) {
	epc, once, stop := Default()
	defer stop()

	for {
		items, canceled := epc.Next(filter)

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
func RunWithIterator(filter *localnetv1.EndpointConditions, handler func(*Iterator)) {
	epc, once, stop := Default()
	defer stop()

	for {
		iter := epc.NextIterator(filter)

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
