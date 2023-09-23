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

package localv1

import "fmt"

func ExampleEndpointPortMapping() {
	ports := []*PortMapping{
		// name doesn't match -> ignore the rest
		{Name: "http", TargetPortName: "t-http", TargetPort: 8080},
		// name matches -> ignore the rest
		{Name: "http2", TargetPortName: "t-http2", TargetPort: 800},
		// name matches -> ignore the rest
		{Name: "metrics", TargetPortName: "http2"},
		// name matches -> ignore the rest
		{Name: "metrics", TargetPort: 80},
		// name matches
		{Name: "metrics"},
		// targetPortName matches, no name -> ignore TargetPort
		{TargetPortName: "metrics", TargetPort: 8080},
		// targetPortName doesn't match, no name -> ignore targetPort
		{TargetPortName: "t-metrics", TargetPort: 8080},
		// nothing to match -> err expected
		{},
	}

	ep := &Endpoint{
		PortOverrides: []*PortName{
			{Name: "metrics", Port: 1011},
			{Name: "http2", Port: 888},
		},
	}

	for _, port := range ports {
		p, err := ep.PortMapping(port)
		fmt.Println(port.Name, p, err)
	}

	// Output:
	// http 0 not found http in port overrides
	// http2 888 <nil>
	// metrics 1011 <nil>
	// metrics 1011 <nil>
	// metrics 1011 <nil>
	//  1011 <nil>
	//  0 not found t-metrics in port overrides
	//  0 port mapping is undefined
}
