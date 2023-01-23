/*
Copyright 2023 The Kubernetes Authors.

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

package ipvsfullsate

import (
	"sigs.k8s.io/kpng/api/localv1"
)

type ServiceType string

const (
	ClusterIPService    ServiceType = "ClusterIP"
	NodePortService     ServiceType = "NodePort"
	LoadBalancerService ServiceType = "LoadBalancer"
)

// String returns ServiceType as string.
func (st ServiceType) String() string {
	return string(st)
}

// SessionAffinity contains data about assigned session affinity.
type SessionAffinity struct {
	ClientIP *localv1.Service_ClientIP
}
