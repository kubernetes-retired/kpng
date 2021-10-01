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
