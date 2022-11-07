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

	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/api/globalv1"
)

func Example() {
	s := New()

	endpoint := func(ip string, ready bool) *globalv1.EndpointInfo {
		return &globalv1.EndpointInfo{
			Namespace:   "default",
			SourceName:  "svc0",
			ServiceName: "svc0",
			Conditions: &globalv1.EndpointConditions{
				Ready: ready,
			},
			Endpoint: &localv1.Endpoint{
				IPs: &localv1.IPSet{V4: []string{ip}},
			},
		}
	}

	for _, twoReady := range []bool{true, false} {
		s.Update(func(tx *Tx) {
			tx.SetService(&localv1.Service{
				Namespace: "default",
				Name:      "svc0",
			})

			tx.SetEndpointsOfSource("default", "svc0", []*globalv1.EndpointInfo{
				endpoint("10.0.0.1", false),
				endpoint("10.0.0.2", twoReady),
			})
		})

		fmt.Println("two ready:", twoReady)
		infos := make([]*globalv1.EndpointInfo, 0)
		s.View(0, func(tx *Tx) {
			tx.EachEndpointOfService("default", "svc0", func(info *globalv1.EndpointInfo) {
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
		tx.SetService(&localv1.Service{
			Namespace: "test",
			Name:      "session-affinity-service",
			Type:      "NodePort",
			IPs:       &localv1.ServiceIPs{ClusterIPs: localv1.NewIPSet("10.1.2.5")},
			SessionAffinity: &localv1.Service_ClientIP{
				ClientIP: &localv1.ClientIPAffinity{
					TimeoutSeconds: 10800,
				}},
			ExternalTrafficToLocal: false,
			Labels:                 map[string]string{"selector-48a01edd-8df9-4826-868a-945ccf3e932a": "true"},
			Ports: []*localv1.PortMapping{
				{Name: "http",
					NodePort:   31279,
					Protocol:   localv1.Protocol_TCP,
					Port:       80,
					TargetPort: 8083},
				{Name: "udp",
					NodePort:   30024,
					Protocol:   localv1.Protocol_UDP,
					Port:       90,
					TargetPort: 8081},
			},
		})

		tx.SetEndpointsOfSource("test", "test-abcde", []*globalv1.EndpointInfo{
			{
				Namespace:   "test",
				SourceName:  "test-abcde",
				ServiceName: "session-affinity-service",
				Endpoint:    &localv1.Endpoint{IPs: localv1.NewIPSet("10.2.0.1")},
				Conditions:  &globalv1.EndpointConditions{Ready: true},
			},
		})
	})
}
