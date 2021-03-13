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

package endpoints

import (
	"k8s.io/client-go/tools/cache"

	"m.cluseau.fr/kpng/pkg/proxy"
	"m.cluseau.fr/kpng/pkg/proxystore"
)

type Correlator struct {
	proxy *proxy.Server
	store *proxystore.Store
}

func NewCorrelator(proxyServer *proxy.Server) *Correlator {
	return &Correlator{
		proxy: proxyServer,
		store: proxyServer.Store,
	}
}

func (c *Correlator) eventHandler(informer cache.SharedIndexInformer) eventHandler {
	return eventHandler{
		s:        c.store,
		informer: informer,
	}
}

func (c *Correlator) Start(stopCh chan struct{}) {
	factory := c.proxy.InformerFactory
	coreFactory := factory.Core().V1()

	{
		servicesInformer := coreFactory.Services().Informer()
		servicesInformer.AddEventHandler(&serviceEventHandler{c.eventHandler(servicesInformer)})
		go servicesInformer.Run(stopCh)

		nodesInformer := coreFactory.Nodes().Informer()
		nodesInformer.AddEventHandler(&nodeEventHandler{c.eventHandler(nodesInformer)})
		go nodesInformer.Run(stopCh)
	}

	if proxy.ManageEndpointSlices {
		slicesInformer := factory.Discovery().V1beta1().EndpointSlices().Informer()
		slicesInformer.AddEventHandler(&sliceEventHandler{c.eventHandler(slicesInformer)})
		go slicesInformer.Run(stopCh)

	} else {
		endpointsInformer := coreFactory.Endpoints().Informer()
		endpointsInformer.AddEventHandler(&endpointsEventHandler{c.eventHandler(endpointsInformer)})
		go endpointsInformer.Run(stopCh)
	}
}
