package localnetv1

import "fmt"

func ExampleEndpointPortMapping() {
	ports := []*PortMapping{
		{Name: "http", TargetPortName: "t-http", TargetPort: 8080},
		{Name: "http2", TargetPortName: "t-http2", TargetPort: 800},
		{Name: "metrics", TargetPortName: "t-metrics"},
	}

	ep := &Endpoint{
		PortOverrides: map[string]int32{"metrics": 1011, "http2": 888},
	}

	for _, port := range ports {
		fmt.Println(port.Name, ep.PortMapping(port))
	}

	// Output:
	// http 8080
	// http2 888
	// metrics 1011
}
