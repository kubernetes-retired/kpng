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
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/discovery/v1beta1"
)

type correlationSource struct {
	Service   *v1.Service
	Endpoints *v1.Endpoints
	Slices    []*v1beta1.EndpointSlice
}

func (cs correlationSource) IsEmpty() bool {
	return cs.Service == nil && cs.Endpoints == nil && len(cs.Slices) == 0
}
