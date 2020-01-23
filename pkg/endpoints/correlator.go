package endpoints

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/cespare/xxhash"
	"github.com/gogo/protobuf/proto"
	"github.com/google/btree"
	"k8s.io/klog"

	"github.com/mcluseau/kube-localnet-api/pkg/api/localnetv1"
	"github.com/mcluseau/kube-localnet-api/pkg/proxy"
)

type Correlator struct {
	proxy *proxy.Server
	rwL   *sync.RWMutex
	cond  *sync.Cond
	rev   uint64

	synced                bool
	servicesSynced        bool
	endpointsSynced       bool
	endpointsSlicesSynced bool

	eventL     sync.Mutex
	eventCount int

	// index to correlate service-related resource by services' namespace/name
	sources map[string]correlationSource

	endpoints *btree.BTree

	// slices shouldn't change service, but this would allow managing that case
	// sliceService map[string]string
}

func NewCorrelator(proxyServer *proxy.Server) *Correlator {
	return &Correlator{
		proxy:     proxyServer,
		rwL:       &sync.RWMutex{},
		cond:      sync.NewCond(&sync.Mutex{}),
		sources:   make(map[string]correlationSource),
		endpoints: btree.New(2),
	}
}

func (c *Correlator) WaitRev(lastKnownRev uint64) {
	c.cond.L.Lock()
	for c.rev <= lastKnownRev {
		c.cond.Wait()
	}
	c.cond.L.Unlock()
}

func (c *Correlator) Next(lastKnownRev uint64) (results []*localnetv1.ServiceEndpoints) {
	c.WaitRev(lastKnownRev)

	c.rwL.RLock()
	defer c.rwL.RUnlock()

	results = make([]*localnetv1.ServiceEndpoints, 0, c.endpoints.Len())

	c.endpoints.Ascend(func(i btree.Item) bool {
		kv := i.(endpointsKV)
		results = append(results, kv.Endpoints)
		return true
	})

	return
}

func (c *Correlator) Run(stopCh chan struct{}) {
	factory := c.proxy.InformerFactory
	coreFactory := factory.Core().V1()

	{
		svcInformer := coreFactory.Services().Informer()
		svcInformer.AddEventHandler(serviceEventHandler{c, &c.servicesSynced, svcInformer})
		go svcInformer.Run(stopCh)
	}

	if proxy.ManageEndpointSlices {
		c.endpointsSynced = true // not going to watch them

		sliceInformer := factory.Discovery().V1beta1().EndpointSlices().Informer()
		sliceInformer.AddEventHandler(sliceEventHandler{c, &c.endpointsSlicesSynced, sliceInformer})
		go sliceInformer.Run(stopCh)

	} else {
		c.endpointsSlicesSynced = true // not going to watch them

		epInformer := coreFactory.Endpoints().Informer()
		epInformer.AddEventHandler(endpointsEventHandler{c, &c.endpointsSynced, epInformer})
		go epInformer.Run(stopCh)
	}

	go c.logStats()
}

func (c *Correlator) logStats() {
	evtCount := 0

	rusage := &syscall.Rusage{}
	memStats := &runtime.MemStats{}

	syscall.Getrusage(syscall.RUSAGE_SELF, rusage)
	prevSys := rusage.Stime.Nano()
	prevUsr := rusage.Utime.Nano()

	t0 := time.Now()
	prevTime := time.Time{}

	tick := time.Tick(time.Second)
	fmt.Println("stats:\ttime\tevents\trev\tusr cpu\tsys cpu\ttot cpu\tmem\trevs/events")
	fmt.Println("stats:\tms\tcount\tcount\tms\tms\t%\tMiB\t%")
	for {
		evtCount = c.eventCount

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

		fmt.Printf("stats:\t%d\t%d\t%d\t%d\t%d\t%.3f\t%.2f\t%.3f\n",
			time.Since(t0).Milliseconds(),
			evtCount,
			c.rev,
			sysD/1e6,
			usrD/1e6,
			float64(sysD+usrD)*100/float64(elapsed),
			float64(memStats.Alloc)/(2<<20),
			float64(c.rev*100)/float64(evtCount),
		)

		prevTime = now
		prevSys = sys
		prevUsr = usr

		<-tick
	}
}

func (c *Correlator) afterEvent(namespace, name string) {
	c.eventCount++

	synced := c.servicesSynced &&
		c.endpointsSynced &&
		c.endpointsSlicesSynced

	source := c.sources[namespace+"/"+name]

	epKV := endpointsKV{
		Namespace: namespace,
		Name:      name,
		Endpoints: computeServiceEndpoints(source),
	}

	updated := false

	// TODO later: batch tree updates

	// fetch current item
	item := c.endpoints.Get(epKV)

	if epKV.Endpoints == nil {
		// deleted
		if item != nil {
			c.rwL.Lock()
			defer c.rwL.Unlock()

			klog.V(1).Infof("endpoints removed: %s/%s", namespace, name)
			c.endpoints.Delete(item)

			updated = true
		}

	} else {
		// created or updated
		encoded, err := proto.Marshal(epKV.Endpoints) // TODO use a cached proto.NewBuffer
		if err != nil {
			klog.Errorf("failed to marshal endpoints for %s/%s: %v", namespace, name, err)
			return
		}

		h := xxhash.Sum64(encoded)

		if item == nil || item.(endpointsKV).EndpointsHash != h {
			epKV.EndpointsHash = h

			c.rwL.Lock()
			defer c.rwL.Unlock()

			klog.V(1).Infof("endpoints updated: %s/%s", namespace, name)
			c.endpoints.ReplaceOrInsert(epKV)

			updated = true
		}
	}

	if (c.synced == synced) && !updated {
		return
	}

	if synced && !c.synced {
		c.synced = true
		klog.Info("all informers are synced")
	}

	c.cond.L.Lock()
	c.rev++
	c.cond.L.Unlock()
	c.cond.Broadcast()
}
