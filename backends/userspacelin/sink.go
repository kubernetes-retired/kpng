package userspacelin

import (
	"io"
	"log"
	"time"

	utilnet "k8s.io/apimachinery/pkg/util/net"

	"k8s.io/utils/exec"
	iptablesutil "sigs.k8s.io/kpng/backends/iptables/util"

	v1 "k8s.io/api/core/v1"

	"sync"

	"github.com/spf13/pflag"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
)

type Backend struct {
	localsink.Config

	services  map[string]*service
	ips       map[string]bool
	listeners map[string]io.Closer
}

var wg = sync.WaitGroup{}
var usImpl map[v1.IPFamily]*UserspaceLinux
var hostname string
var _ decoder.Interface = &Backend{}

func New() *Backend {
	return &Backend{}
}

func (s *Backend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(s))
}

func (s *Backend) BindFlags(flags *pflag.FlagSet) {
}

func (s *Backend) Setup() {
	hostname = s.NodeName
	// make a proxier for ipv4, ipv6

	usImpl = make(map[v1.IPFamily]*UserspaceLinux)
	execer := exec.New()
	for _, protocol := range []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol} {
		theProxier, err := NewUserspaceLinux(
			NewLoadBalancerRR(),
			utilnet.ParseIPSloppy("0.0.0.0"),
			iptablesutil.Interface,
			execer,
			utilnet.PortRange{Base: 30000, Size: 2768},
			time.Duration(15),
			time.Duration(15),
			time.Duration(10),
		)
		if err != nil {
			log.Fatal("unable to create proxier: %v", err)
		} else {
			usImpl[protocol] = theProxier
		}
	}
}

func (s *Backend) Reset() { /* noop, we're wrapped in filterreset */ }

func (s *Backend) Sync() {
	for _, impl := range usImpl {
		wg.Add(1)
		go impl.syncProxyRules()
	}
	wg.Wait()
}

func (s *Backend) SetService(svc *localnetv1.Service) {
	key := svc.NamespacedName()
	for _, impl := range usImpl {
		if oldSvc, ok := s.services[key]; ok {
			impl.OnServiceUpdate(oldSvc.internalSvc, svc)
		} else {
			impl.OnServiceAdd(svc)
		}
		s.services[key] = &service{Name: key, internalSvc: svc}
	}
}

func (s *Backend) DeleteService(namespace, name string) {
	key := namespace + "/" + name
	for _, impl := range usImpl {
		impl.OnServiceDelete(s.services[key].internalSvc)
		delete(s.services, key)
	}
}

// name of the endpoint is the same as the service name
func (s *Backend) SetEndpoint(namespace, serviceName, epKey string, endpoint *localnetv1.Endpoint) {
	svc := s.services[namespace+"/"+serviceName]
	for _, impl := range usImpl {
		svc.AddEndpoint(epKey, endpoint)
		impl.OnEndpointsAdd(endpoint, svc.internalSvc)
	}
}

func (s *Backend) DeleteEndpoint(namespace, serviceName, epKey string) {
	key := namespace + "/" + serviceName
	svc := s.services[key]
	for _, impl := range usImpl {
		if ep := svc.GetEndpoint(epKey); ep.key == epKey {
			impl.OnEndpointsDelete(ep.internalEp, svc.internalSvc)
		}
	}
}

// 1
// 2 <-- last connection sent here
// 3
// -------------------- rr.OnEndpointsUpdate (1,2,3) (1,2,3,4)
// 1
// 2
// 3 <-- next connection will send here
// 4
// --------------------- rr.OnEndpointsUpdate (1,2,3) (1,2,4)
// 1
// 2
// 4 <-- next connection will send here
// ---------------------
// 1 <-- next connection will send here
// 2
// 4
