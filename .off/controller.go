package main

import (
	"sync"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type controller struct {
	client    *kubernetes.Clientset
	informers informers.SharedInformerFactory
}

func (c *controller) Run(stopCh <-chan struct{}) {
	svcInformer := c.informers.Core().V1().Services().Informer()
	epInformer := c.informers.Core().V1().Endpoints().Informer()

	informers := []cache.SharedInformer{
		svcInformer,
		epInformer,
	}

	synced := func() bool {
		// FIXME this is wrong under race conditions, but close enough for a PoC
		for _, informer := range informers {
			if !informer.HasSynced() {
				return false
			}
		}
		return true
	}

	seChanges := make(chan serviceEndpointsChange, 10)
	svcEpCorrelator := &serviceEndpointsCorrelator{
		SvcStore: svcInformer.GetStore(),
		EPStore:  epInformer.GetStore(),
		Synced:   synced,
		Changes:  seChanges,
	}

	go processServiceEndpointsChanges(seChanges)

	svcInformer.AddEventHandler(svcEpCorrelator)
	epInformer.AddEventHandler(svcEpCorrelator)

	wg := sync.WaitGroup{}

	wg.Add(len(informers))
	for _, informer := range informers {
		informer := informer
		go func() {
			defer wg.Done()
			informer.Run(stopCh)
		}()
	}

	wg.Wait()

	close(seChanges)
}
