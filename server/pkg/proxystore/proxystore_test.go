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

package proxystore

import (
	"fmt"
	"sort"
	"testing"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

func Example() {
	s := New()

	endpoint := func(ip string, ready bool) *localnetv1.EndpointInfo {
		return &localnetv1.EndpointInfo{
			Namespace:   "default",
			SourceName:  "svc0",
			ServiceName: "svc0",
			Conditions: &localnetv1.EndpointConditions{
				Ready: ready,
			},
			Endpoint: &localnetv1.Endpoint{
				IPs: &localnetv1.IPSet{V4: []string{ip}},
			},
		}
	}

	for _, twoReady := range []bool{true, false} {
		s.Update(func(tx *Tx) {
			tx.SetService(&localnetv1.Service{
				Namespace: "default",
				Name:      "svc0",
			}, []string{"*"})

			tx.SetEndpointsOfSource("default", "svc0", []*localnetv1.EndpointInfo{
				endpoint("10.0.0.1", false),
				endpoint("10.0.0.2", twoReady),
			})
		})

		fmt.Println("two ready:", twoReady)
		infos := make([]*localnetv1.EndpointInfo, 0)
		s.View(0, func(tx *Tx) {
			tx.EachEndpointOfService("default", "svc0", func(info *localnetv1.EndpointInfo) {
				infos = append(infos, info)
			})
		})

		sort.Slice(infos, func(i, j int) bool { return infos[i].Endpoint.String() < infos[j].Endpoint.String() })

		for _, info := range infos {
			fmt.Println("-", info.Endpoint, "[", info.Conditions, "]")
		}
	}

	// Output:
	// two ready: true
	// - IPs:{V4:"10.0.0.1"} [  ]
	// - IPs:{V4:"10.0.0.2"} [ Ready:true ]
	// two ready: false
	// - IPs:{V4:"10.0.0.1"} [  ]
	// - IPs:{V4:"10.0.0.2"} [  ]

}

// TestSessionAffinitySetClientIP creates scenario to validate SessionAffinity
// Ref: https://github.com/kubernetes-sigs/kpng/issues/156
func TestSessionAffinitySetClientIP(t *testing.T) {
	store := New()
	store.Update(func(tx *Tx) {
		tx.SetService(&localnetv1.Service{
			Namespace: "test",
			Name:      "session-affinity-service",
			Type:      "NodePort",
			IPs:       &localnetv1.ServiceIPs{ClusterIPs: localnetv1.NewIPSet("10.1.2.5")},
			SessionAffinity: &localnetv1.Service_ClientIP{
				ClientIP: &localnetv1.ClientIPAffinity{
					TimeoutSeconds: 10800,
				}},
			ExternalTrafficToLocal: false,
			Labels:                 map[string]string{"selector-48a01edd-8df9-4826-868a-945ccf3e932a": "true"},
			Ports: []*localnetv1.PortMapping{
				{Name: "http",
					NodePort:   31279,
					Protocol:   localnetv1.Protocol_TCP,
					Port:       80,
					TargetPort: 8083},
				{Name: "udp",
					NodePort:   30024,
					Protocol:   localnetv1.Protocol_UDP,
					Port:       90,
					TargetPort: 8081},
			},
		}, []string{"*"})

		tx.SetEndpointsOfSource("test", "test-abcde", []*localnetv1.EndpointInfo{
			{
				Namespace:   "test",
				SourceName:  "test-abcde",
				ServiceName: "session-affinity-service",
				Endpoint:    &localnetv1.Endpoint{IPs: localnetv1.NewIPSet("10.2.0.1")},
				Conditions:  &localnetv1.EndpointConditions{Ready: true},
			},
		})
	})
}
