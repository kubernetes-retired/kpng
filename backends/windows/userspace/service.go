/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package userspace

import "sigs.k8s.io/kpng/api/localv1"

// service is the operational view of a service for userspace-proxing
type service struct {
	Name        string
	eps         []endpoint
	internalSvc *localv1.Service
}

// endpoint is the operational view of a service endpoint
type endpoint struct {
	key        string
	targetIP   string
	internalEp *localv1.Endpoint
}

func (svc *service) AddEndpoint(key string, ep *localv1.Endpoint) {
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
