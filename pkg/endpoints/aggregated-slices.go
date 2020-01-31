package endpoints

import (
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1beta1"
)

type aggregatedSlices struct {
	Slices []*discovery.EndpointSlice
	byKey  map[uint64]*discovery.EndpointSlice
}

func newAggregatedSlices() *aggregatedSlices {
	return &aggregatedSlices{
		Slices: make([]*discovery.EndpointSlice, 0),
		byKey:  map[uint64]*discovery.EndpointSlice{},
	}
}

func (agg *aggregatedSlices) SliceFromCoreV1(sk *SliceKey) *discovery.EndpointSlice {
	h := sk.Hash()

	if slice, ok := agg.byKey[h]; ok {
		return slice
	}

	ports := make([]discovery.EndpointPort, len(sk.Ports))
	for idx, port := range sk.Ports {
		protocol := v1.Protocol(port.Protocol.String())
		ports[idx] = discovery.EndpointPort{
			Name:     &port.Name,
			Protocol: &protocol,
			Port:     &port.Port,
			// XXX and AppProtocol?
		}
	}

	slice := &discovery.EndpointSlice{
		Ports:       ports,
		AddressType: discovery.AddressType(sk.AddressType),
		Endpoints: []discovery.Endpoint{
			{
				Topology: sk.Topology,
				Conditions: discovery.EndpointConditions{
					Ready: &_true,
				},
			},
		},
	}

	agg.Slices = append(agg.Slices, slice)
	agg.byKey[h] = slice

	return slice
}

func (agg *aggregatedSlices) AddCoreV1(svc *v1.Service, subset *v1.EndpointSubset, addr *v1.EndpointAddress,
	nodeLabels map[string]string,
) {
	slice := agg.sliceForKey(SliceKeyFromCoreV1(svc, subset, nodeLabels))
}
