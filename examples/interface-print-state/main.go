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

package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/server/jobs/kube2store"
	"sigs.k8s.io/kpng/server/jobs/store2localinterface"
)

type myBackend struct{}

var _ store2localinterface.NodeLocalStateConsumer = myBackend{}

func (m myBackend) UpdateServices(services <-chan *localnetv1.Service) {
	klog.V(2).Info("In Backend Update Service")
	for service := range services {
		klog.Infof("Got Service Update: Name -> %s, Namespace -> %s\n", service.Name, service.Namespace)
	}
}

func (m myBackend) DeleteServices(services <-chan *localnetv1.Service) {
	klog.V(2).Info("In Backend Delete Service")

	for service := range services {
		klog.Infof("Got Service Delete: %v\n", service)
	}
}

func (m myBackend) UpdateEndpoints(endpoints <-chan *localnetv1.EndpointInfo) {
	klog.V(2).Info("In Backend Update Endpoint")

	for endpoint := range endpoints {
		klog.Infof("Got Endpoint Update: [EPSName: %s, Ips: %+v, isLocal: %v] for Service: %s \n",
			endpoint.SourceName, endpoint.Endpoint.IPs, endpoint.Endpoint.Local, endpoint.ServiceName)
	}
}

func (m myBackend) DeleteEndpoints(endpoints <-chan *localnetv1.EndpointInfo) {
	klog.V(2).Info("In Backend Delete Endpoint")

	for endpoint := range endpoints {
		klog.Infof("Got Endpoint Delete: %v\n", endpoint)
	}
}

var (
	nodeName   = flag.String("nodename", "", "Node Name to get services data")
	kubeconfig = flag.String("kubeconfig", "", "Path to Kube Config")
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	flag.Parse()
	printBackend := myBackend{}
	k2sCfg := &kube2store.Config{}
	// kubeconfig := "~/.kube/config"
	// kubeserver := ""

	ctx, store, err := store2localinterface.StartKube2store(k2sCfg, *kubeconfig, "")
	if err != nil {
		fmt.Printf("Unable to start kube2store exiting %v", err)
		os.Exit(1)
	}
	// If this returns something broke :-)
	store2localinterface.WatchStore(ctx, store, *nodeName, printBackend)

	ctx.Done()
}
