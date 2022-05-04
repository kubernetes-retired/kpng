package userspacelin

import "sigs.k8s.io/kpng/api/localnetv1"

// service is the operational view of a service for userspace-proxing
type service struct {
	Name        string
	eps         []endpoint
	internalSvc *localnetv1.Service
}

// endpoint is the operational view of a service endpoint
type endpoint struct {
	key        string
	targetIP   string
	internalEp *localnetv1.Endpoint
}

func (svc *service) AddEndpoint(key string, ep *localnetv1.Endpoint) {
	if ep.IPs.IsEmpty() { // no IPs on endpoint
		return
	}

	svc.eps = append(svc.eps, endpoint{
		key:        key,
		targetIP:   ep.IPs.First(),
		internalEp: ep,
	})
}

func (svc *service) GetEndpoint(key string) endpoint {
	for _, ep := range svc.eps {
		if ep.key == key {
			return ep
		}
	}
	return endpoint{}
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
