package endpoints

import (
	"k8s.io/client-go/tools/cache"

	"m.cluseau.fr/kube-proxy2/pkg/proxy"
	"m.cluseau.fr/kube-proxy2/pkg/proxystore"
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

func (c *Correlator) Run(stopCh chan struct{}) {
	factory := c.proxy.InformerFactory
	coreFactory := factory.Core().V1()

	{
		servicesInformer := coreFactory.Services().Informer()
		servicesInformer.AddEventHandler(&serviceEventHandler{
			eventHandler: eventHandler{
				s:        c.store,
				informer: svcInformer,
			},
		})
		go servicesInformer.Run(stopCh)

		nodesInformer := coreFactory.Nodes().Informer()
		nodesInformer.AddEventHandler(&nodeEventHandler{c.eventHandler(nodesInformer)})
		go nodesInformer.Run(stopCh)
	}

	if proxy.ManageEndpointSlices {
		slicesInformer := factory.Discovery().V1beta1().EndpointSlices().Informer()
		slicesInformer.AddEventHandler(&sliceEventHandler{c.eventHandler(sliceInformer)})
		go slicesInformer.Run(stopCh)

	} else {
		endpointsInformer := coreFactory.Endpoints().Informer()
		endpointsInformer.AddEventHandler(&endpointsEventHandler{c.eventHandler(epInformer)})
		go endpointsInformer.Run(stopCh)
	}
}
