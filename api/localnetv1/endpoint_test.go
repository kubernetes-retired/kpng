package localnetv1

import "fmt"

func ExampleEndpointPortMapping() {
	ports := []*PortMapping{
		{TargetPortName: "http", TargetPort: 8080},
		{TargetPortName: "http2", TargetPort: 800},
		{TargetPortName: "metrics"},
	}

	ep := &Endpoint{
		PortOverrides: map[string]int32{"metrics": 1011, "http2": 888},
	}

	for _, port := range ports {
		fmt.Println(port.TargetPortName, ep.PortMapping(port))
	}

	// Output:
	// http 8080
	// http2 888
	// metrics 1011
}
