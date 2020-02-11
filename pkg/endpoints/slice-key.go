package endpoints

import (
	"github.com/cespare/xxhash"
	"github.com/gogo/protobuf/proto"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1beta1"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

func SliceKeyFromCoreV1(svc *v1.Service, subset *v1.EndpointSubset, nodeLabels map[string]string) *SliceKey {
	// compute topology
	topology := make(map[string]string, len(svc.Spec.TopologyKeys))

	for _, key := range svc.Spec.TopologyKeys {
		value, ok := nodeLabels[key]

		if !ok {
			continue
		}

		topology[key] = value
	}

	// compute ports
	ports := make([]*localnetv1.Port, len(subset.Ports))
	for idx, port := range subset.Ports {
		ports[idx] = &localnetv1.Port{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: localnetv1.ParseProtocol(string(port.Protocol)),
		}
	}

	// return the key
	return &SliceKey{
		Ports: ports,
	}
}

func SliceKeyFromDiscoveryV1(slice *discovery.EndpointSlice, endpoint *discovery.Endpoint) *SliceKey {
	// compute ports
	ports := make([]*localnetv1.Port, len(slice.Ports))
	for idx, port := range slice.Ports {
		p := &localnetv1.Port{}
		if port.Name != nil {
			p.Name = *port.Name
		}
		if port.Port != nil {
			p.Port = *port.Port
		}
		if port.Protocol != nil {
			p.Protocol = localnetv1.ParseProtocol(string(*port.Protocol))
		}
		ports[idx] = p
	}

	// return the key
	return &SliceKey{
		Ports: ports,
	}
}

func (sk *SliceKey) Hash() uint64 {
	ba, err := proto.Marshal(sk)
	if err != nil {
		panic(err) // not expected to happen
	}

	return xxhash.Sum64(ba)
}
