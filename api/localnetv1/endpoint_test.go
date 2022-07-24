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
