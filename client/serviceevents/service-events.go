package serviceevents

import "sigs.k8s.io/kpng/api/localnetv1"

type IPKind int

const (
	ClusterIP IPKind = iota
	ExternalIP
	LoadBalancerIP
)

type PortsListener interface {
	AddPort(svc *localnetv1.Service, port *localnetv1.PortMapping)
	DeletePort(svc *localnetv1.Service, port *localnetv1.PortMapping)
}

type IPsListener interface {
	AddIP(svc *localnetv1.Service, ip string, ipKind IPKind, port *localnetv1.PortMapping)
	DeleteIP(svc *localnetv1.Service, ip string, ipKind IPKind, port *localnetv1.PortMapping)
}

type IPPortsListener interface {
	AddIPPort(svc *localnetv1.Service, ip string, ipKind IPKind, port *localnetv1.PortMapping)
	DeleteIPPort(svc *localnetv1.Service, ip string, ipKind IPKind, port *localnetv1.PortMapping)
}

type ServicesListener struct {
	PortsListener   PortsListener
	IPsListener     IPsListener
	IPPortsListener IPPortsListener

	services map[string]*localnetv1.Service
}

func New() *ServicesListener {
	return &ServicesListener{
		services: map[string]*localnetv1.Service{},
	}
}

// SetService is called when a service is added or updated
func (sl *ServicesListener) SetService(svc *localnetv1.Service) {
	svcKey := svc.Namespace + "/" + svc.Name

	prevSvc, ok := sl.services[svcKey]

	sl.services[svcKey] = svc

	if !ok {
		// new service

		if sl.PortsListener != nil {
			for _, port := range svc.Ports {
				sl.PortsListener.AddPort(svc, port)
			}
		}

		return
	}

	// updated service
	if sl.PortsListener != nil {
	portsLoop:
		for _, port := range svc.Ports {
			for _, prevPort := range prevSvc.Ports {
				if samePort(port, prevPort) {
					continue portsLoop
				}
			}

			sl.PortsListener.AddPort(svc, port)
		}

	prevPortsLoop:
		for _, prevPort := range prevSvc.Ports {
			for _, port := range svc.Ports {
				if samePort(port, prevPort) {
					continue prevPortsLoop
				}
			}

			sl.PortsListener.DeletePort(svc, prevPort)
		}
	}

}

// DeleteService is called when a service is deleted
func (sl *ServicesListener) DeleteService(namespace, name string) {
	svcKey := namespace + "/" + name
	svc, ok := sl.services[svcKey]
	if !ok {
		return // already removed
	}

	delete(sl.services, svcKey)

	if sl.PortsListener != nil {
		for _, port := range svc.Ports {
			sl.PortsListener.DeletePort(svc, port)
		}
	}
}

func samePort(p1, p2 *localnetv1.PortMapping) bool {
	return p1.Protocol == p2.Protocol && p1.Port == p2.Port
}
