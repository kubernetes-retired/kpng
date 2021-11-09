package main

import (
	"math/rand"
	"time"

	"sigs.k8s.io/kpng/api/localnetv1"
)

func init() {
	// we want to seed the rng
	rand.Seed(time.Now().UnixNano())
}

// service is the operational view of a service for userspace-proxing
type service struct {
	Name string
	eps  []endpoint
}

// endpoint is the operational view of a service endpoint
type endpoint struct {
	key      string
	targetIP string
}

func (svc *service) RandomEndpoint() string {
	eps := svc.eps // eps array is always replaced so no locking is needed

	if len(eps) == 0 {
		return ""
	}

	return eps[rand.Intn(len(eps))].targetIP
}

func (svc *service) AddEndpoint(key string, ep *localnetv1.Endpoint) {
	if ep.IPs.IsEmpty() {
		return
	}

	svc.eps = append(svc.eps, endpoint{
		key:      key,
		targetIP: ep.IPs.First(),
	})
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
