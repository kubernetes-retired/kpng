package endpoints

import (
	"fmt"

	localnetv12 "sigs.k8s.io/kpng/api/localnetv1"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
)

func ExampleForNodeWithTopology() {
	store := proxystore2.New()

	store.Update(func(tx *proxystore2.Tx) {
		tx.SetService(&localnetv12.Service{
			Namespace: "test",
			Name:      "test",
			Type:      "ClusterIP",
			IPs:       &localnetv12.ServiceIPs{ClusterIPs: localnetv12.NewIPSet("10.1.2.3")},
			Ports: []*localnetv12.PortMapping{
				{Port: 1234},
			},
		}, []string{"kubernetes.io/hostname"})

		tx.SetEndpointsOfSource("test", "test-abcde", []*localnetv12.EndpointInfo{
			{
				Namespace:   "test",
				SourceName:  "test-abcde",
				ServiceName: "test",
				Endpoint:    &localnetv12.Endpoint{IPs: localnetv12.NewIPSet("10.2.0.1")},
				Topology:    map[string]string{"kubernetes.io/hostname": "host-a"},
				Conditions:  &localnetv12.EndpointConditions{Ready: true},
			},
			{
				Namespace:   "test",
				SourceName:  "test-abcde",
				ServiceName: "test",
				Endpoint:    &localnetv12.Endpoint{IPs: localnetv12.NewIPSet("10.2.1.1")},
				Topology:    map[string]string{"kubernetes.io/hostname": "host-b"},
				Conditions:  &localnetv12.EndpointConditions{Ready: true},
			},
		})
	})

	store.View(0, func(tx *proxystore2.Tx) {
		tx.Each(proxystore2.Services, func(kv *proxystore2.KV) (cont bool) {
			fmt.Print("service ", kv.Name, ":\n")
			tx.EachEndpointOfService("test", "test", func(epi *localnetv12.EndpointInfo) {
				fmt.Print("  - ep ", epi.Endpoint, " (", epi.Topology, ")\n")
			})
			return true
		})

		for _, host := range []string{"host-a", "host-b"} {
			fmt.Print("host ", host, ":\n")
			tx.Each(proxystore2.Services, func(kv *proxystore2.KV) (cont bool) {
				fmt.Print("  - service ", kv.Name, ":\n")
				for _, epi := range ForNode(tx, kv.Service, host) {
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
