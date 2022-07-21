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

package endpoints

import (
	"fmt"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	proxystore "sigs.k8s.io/kpng/server/pkg/proxystore"
)

func ExampleForNodeWithTopology() {
	store := proxystore.New()

	store.Update(func(tx *proxystore.Tx) {
		tx.SetService(&localnetv1.Service{
			Namespace: "test",
			Name:      "test",
			Type:      "ClusterIP",
			IPs:       &localnetv1.ServiceIPs{ClusterIPs: localnetv1.NewIPSet("10.1.2.3")},
			Ports: []*localnetv1.PortMapping{
				{Port: 1234},
			},

			InternalTrafficToLocal: true,
		})

		tx.SetEndpointsOfSource("test", "test-abcde", []*localnetv1.EndpointInfo{
			{
				Namespace:   "test",
				SourceName:  "test-abcde",
				ServiceName: "test",
				Endpoint:    &localnetv1.Endpoint{IPs: localnetv1.NewIPSet("10.2.0.1")},
				Topology:    map[string]string{"kubernetes.io/hostname": "host-a"},
				Conditions:  &localnetv1.EndpointConditions{Ready: true},
			},
			{
				Namespace:   "test",
				SourceName:  "test-abcde",
				ServiceName: "test",
				Endpoint:    &localnetv1.Endpoint{IPs: localnetv1.NewIPSet("10.2.1.1")},
				Topology:    map[string]string{"kubernetes.io/hostname": "host-b"},
				Conditions:  &localnetv1.EndpointConditions{Ready: true},
			},
		})
	})

	store.View(0, func(tx *proxystore.Tx) {
		tx.Each(proxystore.Services, func(kv *proxystore.KV) (cont bool) {
			fmt.Print("service ", kv.Name, ":\n")
			tx.EachEndpointOfService("test", "test", func(epi *localnetv1.EndpointInfo) {
				fmt.Print("  - ep ", epi.Endpoint, " (", epi.Topology, ")\n")
			})
			return true
		})

		for _, host := range []string{"host-a", "host-b"} {
			fmt.Print("host ", host, ":\n")
			tx.Each(proxystore.Services, func(kv *proxystore.KV) (cont bool) {
				fmt.Print("  - service ", kv.Name, ":\n")

				endpoints, _ /* TODO external endpoints */ := ForNode(tx, kv.Service, host)
				for _, epi := range endpoints {
					fmt.Print("    - ep ", epi.Endpoint.IPs, "\n")
				}
				return true
			})
		}
	})

	// Output:
	// service test:
	//   - ep IPs:{V4:"10.2.1.1"} (map[kubernetes.io/hostname:host-b])
	//   - ep IPs:{V4:"10.2.0.1"} (map[kubernetes.io/hostname:host-a])
	// host host-a:
	//   - service test:
	//     - ep V4:"10.2.0.1"
	// host host-b:
	//   - service test:
	//     - ep V4:"10.2.1.1"
}
