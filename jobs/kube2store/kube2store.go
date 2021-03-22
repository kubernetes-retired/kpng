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

package kube2store

import (
	"context"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"m.cluseau.fr/kpng/pkg/proxystore"
)

type Config struct {
	UseSlices bool

	ServiceLabelGlobs      []string
	ServiceAnnonationGlobs []string

	NodeLabelGlobs      []string
	NodeAnnotationGlobs []string
}

func (c *Config) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&c.UseSlices, "use-slices", true, "use EndpointsSlice (not Endpoints)")

	flags.StringSliceVar(&c.ServiceLabelGlobs, "with-service-labels", nil, "service labels to include")
	flags.StringSliceVar(&c.ServiceAnnonationGlobs, "with-service-annotations", nil, "service annotations to include")

	flags.StringSliceVar(&c.NodeLabelGlobs, "with-node-labels", nil, "node labels to include")
	flags.StringSliceVar(&c.NodeAnnotationGlobs, "with-node-annotations", nil, "node annotations to include")
}

type Job struct {
	Kube   *kubernetes.Clientset
	Store  *proxystore.Store
	Config *Config
}

func (j Job) Run(ctx context.Context) {
	stopCh := ctx.Done()

	// start informers
	factory := informers.NewSharedInformerFactory(j.Kube, time.Second*30)
	factory.Start(stopCh)

	// start watches
	coreFactory := factory.Core().V1()

	{
		servicesInformer := coreFactory.Services().Informer()
		servicesInformer.AddEventHandler(&serviceEventHandler{j.eventHandler(servicesInformer)})
		go servicesInformer.Run(stopCh)

		nodesInformer := coreFactory.Nodes().Informer()
		nodesInformer.AddEventHandler(&nodeEventHandler{j.eventHandler(nodesInformer)})
		go nodesInformer.Run(stopCh)
	}

	if j.Config.UseSlices {
		slicesInformer := factory.Discovery().V1beta1().EndpointSlices().Informer()
		slicesInformer.AddEventHandler(&sliceEventHandler{j.eventHandler(slicesInformer)})
		go slicesInformer.Run(stopCh)

	} else {
		endpointsInformer := coreFactory.Endpoints().Informer()
		endpointsInformer.AddEventHandler(&endpointsEventHandler{j.eventHandler(endpointsInformer)})
		go endpointsInformer.Run(stopCh)
	}
}

func (j Job) eventHandler(informer cache.SharedIndexInformer) eventHandler {
	return eventHandler{
		config:   j.Config,
		s:        j.Store,
		informer: informer,
	}
}
