package main

import (
	"io"
	"log"
	"strconv"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/serviceevents"
)

type userspaceBackend struct {
	nodeName  string
	services  map[string]*service
	ips       map[string]bool
	listeners map[string]io.Closer
}

// ------------------------------------------------------------------------
// Decoder sink backend interface
//

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
// (IP, port) listener interface
//

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
