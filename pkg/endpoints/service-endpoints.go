package endpoints

import (
	v1 "k8s.io/api/core/v1"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

var (
	_true  = true
	_false = false
)

const hostNameLabel = "kubernetes.io/hostname"

type NodeInfo struct {
	Labels map[string]string
}

// EndpointInfo contains all information necessary for endpoint selection
type EndpointInfo struct {
	Topology map[string]string
	Endpoint *localnetv1.Endpoint
}

func computeServiceEndpoints(src correlationSource, nodes map[string]NodeInfo, myNodeName string) (seps *localnetv1.ServiceEndpoints) {
	if src.Service == nil {
		return
	}

	svcSpec := src.Service.Spec

	seps = &localnetv1.ServiceEndpoints{
		Namespace: src.Service.Namespace,
		Name:      src.Service.Name,
		Type:      string(svcSpec.Type),
		IPs: &localnetv1.ServiceIPs{
			ClusterIP:   svcSpec.ClusterIP,
			ExternalIPs: svcSpec.ExternalIPs,
		},
		MapAll:                 false, // TODO
		ExternalTrafficToLocal: src.Service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal,
	}

	// ports information
	ports := make([]*localnetv1.PortMapping, len(svcSpec.Ports))

	for idx, port := range svcSpec.Ports {
		ports[idx] = &localnetv1.PortMapping{
			Name:       port.Name,
			NodePort:   port.NodePort,
			Port:       port.Port,
			Protocol:   localnetv1.ParseProtocol(string(port.Protocol)),
			TargetPort: port.TargetPort.IntVal, // FIXME translate name?
		}
	}

	seps.Ports = ports

	// find and normalize endpoints
	infos := make([]EndpointInfo, 0)

	if src.Endpoints != nil {
		// pre-EndpointSlice compat

		for _, subset := range src.Endpoints.Subsets {
			// check ports
			hasAllPorts := len(subset.Ports) == len(ports)

			// add endpoints
			for _, set := range []struct {
				ready     bool
				addresses []v1.EndpointAddress
			}{
				{true, subset.Addresses},
				{false, subset.NotReadyAddresses},
			} {
				for _, addr := range set.addresses {
					info := EndpointInfo{
						Endpoint: &localnetv1.Endpoint{
							Hostname: addr.Hostname,
							Conditions: &localnetv1.EndpointConditions{
								Ready: set.ready && hasAllPorts,
							},
						},
					}

					if addr.NodeName != nil && *addr.NodeName != "" {
						info.Topology = nodes[*addr.NodeName].Labels

						if *addr.NodeName == myNodeName {
							info.Endpoint.Conditions.Local = true
						}
					}

					if addr.IP != "" {
						info.Endpoint.AddAddress(addr.IP) // XXX handle nil result ? (parse error)
					}

					infos = append(infos, info)
				}
			}
		}
	}

	for _, slice := range src.Slices {
		hasAllPorts := len(slice.Ports) == len(ports)

		for _, sliceEndpoint := range slice.Endpoints {
			info := EndpointInfo{
				Topology: sliceEndpoint.Topology,
				Endpoint: &localnetv1.Endpoint{
					Conditions: &localnetv1.EndpointConditions{
						Ready: false,
					},
				},
			}

			if h := sliceEndpoint.Hostname; h != nil {
				info.Endpoint.Hostname = *h
			}

			if r := sliceEndpoint.Conditions.Ready; hasAllPorts && r != nil && *r {
				info.Endpoint.Conditions.Ready = true
			}

			if labels := info.Topology; labels != nil && labels[hostNameLabel] == myNodeName {
				info.Endpoint.Conditions.Local = true
			}

			for _, addr := range sliceEndpoint.Addresses {
				info.Endpoint.AddAddress(addr)
			}

			infos = append(infos, info)
		}
	}

	// compute endpoints selection

	// - filter by topology
	myNode := nodes[myNodeName]

	topologyKeys := src.Service.Spec.TopologyKeys

	if len(topologyKeys) == 0 {
		topologyKeys = []string{"*"}
	}

	for _, topoKey := range topologyKeys {
		ref := ""

		if topoKey != "*" {
			ref = myNode.Labels[topoKey]

			if ref == "" {
				// we do not have that key, skip
				continue
			}
		}

		selected := false
		for _, info := range infos {
			if info.Endpoint.Conditions.Ready && (topoKey == "*" || (info.Topology != nil && info.Topology[topoKey] == ref)) {
				info.Endpoint.Conditions.Selected = true
				selected = true
			}
		}

		if selected {
			break
		}
	}

	// build final endpoints list
	seps.Endpoints = make([]*localnetv1.Endpoint, len(infos))
	for idx, info := range infos {
		seps.Endpoints[idx] = info.Endpoint
	}

	return
}

func addressOrHostname(addr v1.EndpointAddress) (a []string) {
	if addr.IP != "" {
		a = append(a, addr.IP)
	}

	if addr.Hostname != "" {
		a = append(a, addr.Hostname)
	}

	return
}
