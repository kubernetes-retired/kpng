/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"flag"
	"os"
	"runtime/pprof"

	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
	"k8s.io/klog"
)

type HandleFunc func(items []*ServiceEndpoints)
type HandleChFunc func(items <-chan *ServiceEndpoints)

func Default() (epc *EndpointsClient, once bool, nodeName string, stop func()) {
	onceFlag := flag.Bool("once", false, "only one fetch loop")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	flag.StringVar(&nodeName, "node-name", "", "node name to request to the proxy server")

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

	if nodeName == "" {
		var err error
		nodeName, err = os.Hostname()
		if err != nil {
			klog.Fatal("no node-name set and hostname request failed: ", err)
		}
	}

	return
}

// Run the client with the standard options
func Run(req *localnetv1.WatchReq, handlers ...HandleFunc) {
	epc, once, nodeName, stop := Default()
	defer stop()

	if req == nil {
		req = &localnetv1.WatchReq{}
	}
	if req.NodeName == "" {
		req.NodeName = nodeName
	}

	for {
		items, canceled := epc.Next(req)

		if canceled {
			klog.Infof("finished")
			return
		}

		for _, handler := range handlers {
			handler(items)
		}

		if once {
			return
		}
	}
}

// RunCh runs the client with the standard options, using the channeled version of Next.
// It should consume less memory as the dataset is processed as it's read instead of buffered.
// The handler MUST check iter.Err to ensure the dataset was fuly retrieved without error.
func RunCh(req *localnetv1.WatchReq, handler HandleChFunc) {
	epc, once, nodeName, stop := Default()
	defer stop()

	if req == nil {
		req = &localnetv1.WatchReq{}
	}
	if req.NodeName == "" {
		req.NodeName = nodeName
	}

	for {
		ch, canceled := epc.NextCh(req)

		if canceled {
			klog.Infof("finished")
			return
		}

		handler(ch)

		if once {
			return
		}
	}
}
