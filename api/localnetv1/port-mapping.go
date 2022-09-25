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

func (p *PortMapping) SrcPorts() []int32 {
	switch {
	case p.Port == 0 && p.NodePort == 0:
		return []int32{}
	case p.Port != 0 && p.NodePort == 0:
		return []int32{p.Port}
	case p.Port == 0 && p.NodePort != 0:
		return []int32{p.NodePort}
	case p.Port != 0 && p.NodePort != 0:
		return []int32{p.Port, p.NodePort}
	}
	panic("unreachable")
}
