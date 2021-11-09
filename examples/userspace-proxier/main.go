package main

import (
	"flag"
	"io"
	"log"
	"math/rand"
	"strconv"
	"time"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
	"sigs.k8s.io/kpng/client/serviceevents"
)

func main() {
	// seed the rng
	rand.Seed(time.Now().UnixNano())

	// prepare the runner
	runner := client.Runner{}
	runner.BindFlags(flag.CommandLine)

	// prepare the backend
	backend := &userspaceBackend{
		services:  map[string]*service{},
		ips:       map[string]bool{},
		listeners: map[string]io.Closer{},
	}
	backend.BindFlags()

	// parse command line flags
	flag.Parse()

	// setup the backend
	backend.nodeName = runner.NodeName

	// and run!
	runner.RunSink(filterreset.New(decoder.New(serviceevents.Wrap(backend))))
}

type userspaceBackend struct {
	nodeName  string
	services  map[string]*service
	ips       map[string]bool
	listeners map[string]io.Closer
}

// Sync signals an stream sync event
func (b *userspaceBackend) Sync() { /* noop */ }

// WaitRequest see localsink.Sink#WaitRequest
func (b *userspaceBackend) WaitRequest() (nodeName string, err error) {
	return b.nodeName, nil
}

// Reset see localsink.Sink#Reset
func (b *userspaceBackend) Reset() { /* noop */ }

// SetService is called when a service is added or updated
func (b *userspaceBackend) SetService(svc *localnetv1.Service) {
	key := svc.NamespacedName()

	if _, ok := b.services[key]; ok {
		return
	}

	b.services[key] = &service{Name: key}
}

// DeleteService is called when a service is deleted
func (b *userspaceBackend) DeleteService(namespace, name string) {
	delete(b.services, namespace+"/"+name)
}

// SetEndpoint is called when an endpoint is added or updated
func (b *userspaceBackend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	b.services[namespace+"/"+serviceName].AddEndpoint(key, endpoint)
}

// DeleteEndpoint is called when an endpoint is deleted
func (b *userspaceBackend) DeleteEndpoint(namespace, serviceName, key string) {
	b.services[namespace+"/"+serviceName].DeleteEndpoint(key)
}

// ------------------------------------------------------------------------

var _ serviceevents.IPPortsListener = &userspaceBackend{}

func (b *userspaceBackend) AddIPPort(svc *localnetv1.Service, ip string, _ serviceevents.IPKind, port *localnetv1.PortMapping) {
	key := portKey(svc, ip, port)

	ipPort := ip + ":" + strconv.Itoa(int(port.Port))

	switch port.Protocol {
	case localnetv1.Protocol_TCP:
		lsnr := tcpProxy{
			svc:           b.services[svc.NamespacedName()],
			localAddrPort: ipPort,
			targetPort:    strconv.Itoa(int(port.TargetPort)),
		}.Start()

		if lsnr != nil {
			b.listeners[key] = lsnr
		}

	// TODO case localnetv1.Protocol_UDP:
	// TODO case localnetv1.Protocol_SCTP:

	default:
		log.Print("warning: ignoring port on unmanaged protocol ", port.Protocol)
		return
	}
}

func (b *userspaceBackend) DeleteIPPort(svc *localnetv1.Service, ip string, _ serviceevents.IPKind, port *localnetv1.PortMapping) {
	key := portKey(svc, ip, port)

	lsnr, ok := b.listeners[key]
	if !ok {
		return // listen failed so nothing to do
	}

	lsnr.Close()

	delete(b.listeners, key)
}

func portKey(svc *localnetv1.Service, ip string, port *localnetv1.PortMapping) string {
	return svc.NamespacedName() + "@" + port.Protocol.String() + ":" + strconv.Itoa(int(port.Port))
}

// ------------------------------------------------------------------------

type service struct {
	Name string
	eps  []endpoint
}

type endpoint struct {
	key string
	ep  *localnetv1.Endpoint
}

func (svc *service) RandomEndpoint() *localnetv1.Endpoint {
	eps := svc.eps // eps array is always replaced so no locking is needed

	if len(eps) == 0 {
		return nil
	}

	return eps[rand.Intn(len(eps))].ep
}

func (svc *service) AddEndpoint(key string, ep *localnetv1.Endpoint) {
	if ep.IPs.IsEmpty() {
		return
	}

	svc.eps = append(svc.eps, endpoint{key: key, ep: ep})
}

func (svc *service) DeleteEndpoint(key string) {
	// rebuild the endpoints array
	eps := make([]endpoint, 0, len(svc.eps))
	for _, ep := range svc.eps {
		if ep.key == key {
			continue
		}

		eps = append(eps, ep)
	}

	svc.eps = eps
}
