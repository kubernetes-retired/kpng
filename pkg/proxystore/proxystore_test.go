package proxystore

import (
	"fmt"

	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
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

	s.Update(func(tx *Tx) {
		tx.SetService(&localnetv1.Service{
			Namespace: "default",
			Name:      "svc0",
		}, []string{"*"})

		tx.SetEndpointsOfSource("default", "svc0", []*localnetv1.EndpointInfo{
			endpoint("10.0.0.1", false),
			endpoint("10.0.0.2", true),
		})
	})

	s.View(0, func(tx *Tx) {
		tx.EachEndpointOfService("default", "svc0", func(info *localnetv1.EndpointInfo) {
			fmt.Println("-", info.Endpoint, "[", info.Conditions, "]")
		})
	})

	// Output:
	// - IPs:{V4:"10.0.0.1"} [  ]
	// - IPs:{V4:"10.0.0.2"} [ Ready:true ]

}
