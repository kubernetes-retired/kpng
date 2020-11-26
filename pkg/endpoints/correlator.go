package endpoints

import (
	"fmt"
	"runtime"
	"syscall"
	"time"

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
		svcInformer := coreFactory.Services().Informer()
		svcInformer.AddEventHandler(&serviceEventHandler{
			eventHandler: eventHandler{
				s:        c.store,
				informer: svcInformer,
			},
		})
		go svcInformer.Run(stopCh)

		nodesInformer := coreFactory.Nodes().Informer()
		nodesInformer.AddEventHandler(&nodeEventHandler{c.eventHandler(nodesInformer)})
		go nodesInformer.Run(stopCh)
	}

	if proxy.ManageEndpointSlices {
		sliceInformer := factory.Discovery().V1beta1().EndpointSlices().Informer()
		sliceInformer.AddEventHandler(&sliceEventHandler{c.eventHandler(sliceInformer)})
		go sliceInformer.Run(stopCh)

	} else {
		epInformer := coreFactory.Endpoints().Informer()
		epInformer.AddEventHandler(&endpointsEventHandler{c.eventHandler(epInformer)})
		go epInformer.Run(stopCh)
	}

	go c.logStats()
}

func (c *Correlator) logStats() {
	rusage := &syscall.Rusage{}
	memStats := &runtime.MemStats{}

	syscall.Getrusage(syscall.RUSAGE_SELF, rusage)
	prevSys := rusage.Stime.Nano()
	prevUsr := rusage.Utime.Nano()

	t0 := time.Now()
	prevTime := time.Time{}

	tick := time.Tick(time.Second)
	fmt.Println("stats:\ttime\tusr cpu\tsys cpu\ttot cpu\tmem")
	fmt.Println("stats:\tms\tms\tms\t%\tMiB")
	for {
		syscall.Getrusage(syscall.RUSAGE_SELF, rusage)
		runtime.ReadMemStats(memStats)

		now := time.Now()

		sys := rusage.Stime.Nano()
		usr := rusage.Utime.Nano()
		sysD := sys - prevSys
		usrD := usr - prevUsr

		var elapsed int64
		if !prevTime.IsZero() {
			elapsed = now.Sub(prevTime).Nanoseconds()
		}

		fmt.Printf("stats:\t%d\t%d\t%d\t%.3f\t%.2f\n",
			time.Since(t0).Milliseconds(),
			usrD/1e6,
			sysD/1e6,
			float64(sysD+usrD)*100/float64(elapsed),
			float64(memStats.Alloc)/(2<<20),
		)

		prevTime = now
		prevSys = sys
		prevUsr = usr

		<-tick
	}
}
