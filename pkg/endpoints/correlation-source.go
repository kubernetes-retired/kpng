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
