package serviceevents

import (
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink/decoder"
)

type wrapper struct {
	// backend
	decoder.Interface
	// listener
	l *ServicesListener
}

var _ decoder.Interface = wrapper{}

// Wrap a decoder so it receives detailled events depending on which interfaces
// it implements.
//
// A good practice is to ensure your decoder is implementing what you expect
// this way:
//
//     type MyBackend struct { }
//
//     var _ servicevents.PortsListener   = &MyBackend{}
//     var _ servicevents.IPsListener     = &MyBackend{}
//     var _ servicevents.IPPortsListener = &MyBackend{}
//
func Wrap(backend decoder.Interface) decoder.Interface {
	l := New()

	if v, ok := backend.(PortsListener); ok {
		l.PortsListener = v
	}
	if v, ok := backend.(IPsListener); ok {
		l.IPsListener = v
	}
	if v, ok := backend.(IPPortsListener); ok {
		l.IPPortsListener = v
	}

	wrap := wrapper{
		Interface: backend,
		l:         l,
	}
	return wrap
}

func (w wrapper) SetService(service *localnetv1.Service) {
	w.Interface.SetService(service)
	w.l.SetService(service)
}

func (w wrapper) DeleteService(namespace, name string) {
	w.l.DeleteService(namespace, name)
	w.Interface.DeleteService(namespace, name)
}
