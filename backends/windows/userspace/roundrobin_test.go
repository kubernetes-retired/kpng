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

import (
	"net"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kpng/api/localv1"
)

func stringsInSlice(haystack []string, needles ...string) bool {
	for _, needle := range needles {
		found := false
		for i := range haystack {
			if haystack[i] == needle {
				found = true
				break
			}
		}
		if found == false {
			return false
		}
	}
	return true
}

func expectEndpoint(t *testing.T, loadBalancer *LoadBalancerRR, service ServicePortName, expected string, netaddr net.Addr) {
	endpoint, err := loadBalancer.NextEndpoint(service, netaddr, false)
	if err != nil {
		t.Errorf("Didn't find a service for %s, expected %s, failed with: %v", service, expected, err)
	}
	if endpoint != expected {
		t.Errorf("Didn't get expected endpoint for service %s client %v, expected %s, got: %s", service, netaddr, expected, endpoint)
	}
}

func TestLoadBalanceFailsWithNoEndpoints(t *testing.T) {
	loadBalancer := NewLoadBalancerRR()
	svc := ServicePortName{
		NamespacedName: types.NamespacedName{Namespace: "testnamespace", Name: "foo"}, Port: "does-not-exist",
	}
	endpoint, err := loadBalancer.NextEndpoint(svc, nil, false)
	if err == nil {
		t.Errorf("Didn't fail with non-existent service")
	}
	if len(endpoint) != 0 {
		t.Errorf("Got an endpoint")
	}
}

func TestLoadBalanceWorksWithSingleEndpoint(t *testing.T) {
	loadBalancer := NewLoadBalancerRR()

	tests := []struct {
		description string
		endpoints   []string
		namespace   string
		serviceName string
		portName    string
		port        int32
	}{
		{
			description: "creates a new service with three endpoints",
			endpoints:   []string{"ep1", "ep2", "ep3"},
			namespace:   "default",
			serviceName: "foo",
			portName:    "http",
			port:        123,
		},
		{
			description: "creates a new service with one endpoint",
			endpoints:   []string{"ep1"},
			namespace:   "default",
			serviceName: "bar",
			portName:    "https",
			port:        456,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			svcPortName := ServicePortName{
				NamespacedName: types.NamespacedName{Namespace: tt.namespace, Name: tt.serviceName}, Port: tt.portName,
			}

			endpoint, err := loadBalancer.NextEndpoint(svcPortName, nil, false)
			if err == nil || len(endpoint) != 0 {
				t.Errorf("Didn't fail with non-existent service %d %s", len(endpoint), err)
			}
			service := &localv1.Service{Namespace: tt.namespace, Name: tt.serviceName, Ports: []*localv1.PortMapping{
				{Name: tt.portName, Protocol: localv1.Protocol_TCP, Port: tt.port},
			}}

			for _, epAddr := range tt.endpoints {
				ep := &localv1.Endpoint{IPs: &localv1.IPSet{V4: []string{epAddr}}}
				loadBalancer.OnEndpointsAdd(ep, service)
			}

			shuffledEps := loadBalancer.services[svcPortName].endpoints
			if len(shuffledEps) != len(tt.endpoints) {
				t.Errorf("Incorrect number of shuffled endpoints.")
			}

			// iterate 3 times on existent endpoints
			for i := 0; i < 2; i++ {
				for j := 0; j < len(shuffledEps); j++ {
					expectEndpoint(t, loadBalancer, svcPortName, shuffledEps[j], nil)
				}
			}
		})
	}
}

func TestLoadBalanceWorksWithMultipleEndpointsAndUpdates(t *testing.T) {
	loadBalancer := NewLoadBalancerRR()

	serviceP := ServicePortName{NamespacedName: types.NamespacedName{Namespace: "testnamespace", Name: "foo"}, Port: "p"}
	endpoint, err := loadBalancer.NextEndpoint(serviceP, nil, false)
	if err == nil || len(endpoint) != 0 {
		t.Errorf("Didn't fail with non-existent service")
	}

	serviceQ := ServicePortName{NamespacedName: types.NamespacedName{Namespace: "testnamespace", Name: "foo"}, Port: "q"}
	endpoint, err = loadBalancer.NextEndpoint(serviceQ, nil, false)
	if err == nil || len(endpoint) != 0 {
		t.Errorf("Didn't fail with non-existent service %d %s", len(endpoint), err)
	}

	service1 := &localv1.Service{Namespace: "testnamespace", Name: "foo", Ports: []*localv1.PortMapping{
		{Name: "p", Protocol: localv1.Protocol_TCP, Port: 1},
		{Name: "q", Protocol: localv1.Protocol_TCP, Port: 10},
	}}

	endpoint1 := &localv1.Endpoint{IPs: &localv1.IPSet{V4: []string{"endpoint1"}}}
	endpoint2 := &localv1.Endpoint{IPs: &localv1.IPSet{V4: []string{"endpoint2"}}}
	endpoint3 := &localv1.Endpoint{IPs: &localv1.IPSet{V4: []string{"endpoint3"}}}

	service2 := &localv1.Service{Namespace: "testnamespace", Name: "foo", Ports: []*localv1.PortMapping{
		{Name: "q", Protocol: localv1.Protocol_TCP, Port: 456},
		{Name: "q", Protocol: localv1.Protocol_TCP, Port: 678},
	}}

	loadBalancer.OnEndpointsAdd(endpoint1, service1)
	loadBalancer.OnEndpointsAdd(endpoint2, service1)
	loadBalancer.OnEndpointsAdd(endpoint3, service1)
	loadBalancer.OnEndpointsAdd(endpoint3, service2)

	shuffledEndpoints := loadBalancer.services[serviceP].endpoints
	if !stringsInSlice(shuffledEndpoints, "endpoint1:0", "endpoint2:0", "endpoint3:0") {
		t.Errorf("did not find expected endpoints: %v", shuffledEndpoints)
	}
	expectEndpoint(t, loadBalancer, serviceP, shuffledEndpoints[0], nil)
	expectEndpoint(t, loadBalancer, serviceP, shuffledEndpoints[1], nil)
	expectEndpoint(t, loadBalancer, serviceP, shuffledEndpoints[2], nil)
	expectEndpoint(t, loadBalancer, serviceP, shuffledEndpoints[0], nil)

	shuffledEndpoints = loadBalancer.services[serviceQ].endpoints
	if !stringsInSlice(shuffledEndpoints, "endpoint1:0", "endpoint2:0", "endpoint2:0") {
		t.Errorf("did not find expected endpoints: %v", shuffledEndpoints)
	}
	expectEndpoint(t, loadBalancer, serviceQ, shuffledEndpoints[0], nil)
	expectEndpoint(t, loadBalancer, serviceQ, shuffledEndpoints[1], nil)
	expectEndpoint(t, loadBalancer, serviceQ, shuffledEndpoints[2], nil)
	expectEndpoint(t, loadBalancer, serviceQ, shuffledEndpoints[3], nil)
	expectEndpoint(t, loadBalancer, serviceQ, shuffledEndpoints[4], nil)
	expectEndpoint(t, loadBalancer, serviceQ, shuffledEndpoints[0], nil)
}

func TestLoadBalanceWorksWithServiceRemoval(t *testing.T) {
	loadBalancer := NewLoadBalancerRR()

	tests := []struct {
		description       string
		endpoints         []string
		namespace         string
		serviceName       string
		portName          string
		port              int32
		expectedEndpoints int
	}{
		{
			description:       "creates a new service with three endpoints removing one",
			endpoints:         []string{"ep1", "ep2", "ep3"},
			namespace:         "default",
			serviceName:       "foo",
			portName:          "http",
			port:              123,
			expectedEndpoints: 2,
		},
		{
			description:       "creates a new service with one endpoint removing one",
			endpoints:         []string{"ep1"},
			namespace:         "default",
			serviceName:       "bar",
			portName:          "https",
			port:              456,
			expectedEndpoints: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			var lastEp *localv1.Endpoint
			svcPortName := ServicePortName{
				NamespacedName: types.NamespacedName{Namespace: tt.namespace, Name: tt.serviceName}, Port: tt.portName,
			}

			endpoint, err := loadBalancer.NextEndpoint(svcPortName, nil, false)
			if err == nil || len(endpoint) != 0 {
				t.Errorf("Didn't fail with non-existent service %d %s", len(endpoint), err)
			}
			service := &localv1.Service{Namespace: tt.namespace, Name: tt.serviceName, Ports: []*localv1.PortMapping{
				{Name: tt.portName, Protocol: localv1.Protocol_TCP, Port: tt.port},
			}}

			for _, epAddr := range tt.endpoints {
				ep := &localv1.Endpoint{IPs: &localv1.IPSet{V4: []string{epAddr}}}
				loadBalancer.OnEndpointsAdd(ep, service)
				lastEp = ep
			}

			shuffledEps := loadBalancer.services[svcPortName].endpoints
			if len(shuffledEps) != len(tt.endpoints) {
				t.Errorf("Incorrect number of shuffled endpoints.")
			}

			// iterate 3 times on existent endpoints
			for i := 0; i < 2; i++ {
				for j := 0; j < len(shuffledEps); j++ {
					expectEndpoint(t, loadBalancer, svcPortName, shuffledEps[j], nil)
				}
			}

			// remove the first endpoint from service, check length of leftover
			loadBalancer.OnEndpointsDelete(lastEp, service)
			_, err = loadBalancer.NextEndpoint(svcPortName, nil, false)
			lenCurrEps := len(loadBalancer.services[svcPortName].endpoints)
			if lenCurrEps != tt.expectedEndpoints {
				t.Errorf("Did not cleanup the endpointskuhhyhghbfvbhhhhnbnhjvhbgj")
			}
		})
	}
}
