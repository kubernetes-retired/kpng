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
	AddressOrHostname string
	Topology          map[string]string
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
		AllEndpoints:           &localnetv1.EndpointList{},
		SelectedEndpoints:      &localnetv1.EndpointList{},
		LocalEndpoints:         &localnetv1.EndpointList{},
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
	readyEndpoints := make([]EndpointInfo, 0)

	addInfo := func(ready bool, info EndpointInfo) {
		if info.AddressOrHostname == "" {
			return
		}

		if ready {
			readyEndpoints = append(readyEndpoints, info)
		}

		seps.AllEndpoints.Add(info.AddressOrHostname)
	}

	if src.Endpoints != nil {
		// pre-EndpointSlice compat

		for _, subset := range src.Endpoints.Subsets {
			// check ports
			hasAllPorts := len(subset.Ports) == len(ports)

			// ready endpoints
			for _, addr := range subset.Addresses {
				labels := map[string]string{}
				if addr.NodeName != nil && *addr.NodeName != "" {
					labels = nodes[*addr.NodeName].Labels
				}

				for _, addrOrHost := range addressOrHostname(addr) {
					addInfo(hasAllPorts, EndpointInfo{
						AddressOrHostname: addrOrHost,
						Topology:          labels,
					})
				}
			}

			for _, addr := range subset.NotReadyAddresses {
				for _, addrOrHost := range addressOrHostname(addr) {
					seps.AllEndpoints.Add(addrOrHost)
				}
			}
		}
	}

	for _, slice := range src.Slices {
		hasAllPorts := len(slice.Ports) == len(ports)

		for _, sliceEndpoint := range slice.Endpoints {
			ready := false

			if r := sliceEndpoint.Conditions.Ready; r != nil {
				ready = *r
			}

			ready = hasAllPorts && ready

			for _, addr := range sliceEndpoint.Addresses {
				addInfo(ready, EndpointInfo{
					AddressOrHostname: addr,
					Topology:          sliceEndpoint.Topology,
				})
			}

			if hostname := sliceEndpoint.Hostname; hostname != nil && *hostname != "" {
				addInfo(ready, EndpointInfo{
					AddressOrHostname: *hostname,
					Topology:          sliceEndpoint.Topology,
				})
			}
		}
	}

	// compute endpoints selection

	// - filter by topology
	myNode := nodes[myNodeName]

	// only look for things we have
	ipv4Done := len(seps.AllEndpoints.IPsV4) == 0
	ipv6Done := len(seps.AllEndpoints.IPsV6) == 0
	hostnamesDone := len(seps.AllEndpoints.Hostnames) == 0

	epList := &localnetv1.EndpointList{}

	// merge from topology
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

		epList.ResetSets()
		for _, info := range readyEndpoints {
			if topoKey == "*" || info.Topology[topoKey] == ref {
				epList.Add(info.AddressOrHostname)
			}
		}

		// merge endpoints
		if !ipv4Done && len(epList.IPsV4) != 0 {
			seps.SelectedEndpoints.IPsV4 = epList.IPsV4
			ipv4Done = true
		}

		if !ipv6Done && len(epList.IPsV6) != 0 {
			seps.SelectedEndpoints.IPsV6 = epList.IPsV6
			ipv6Done = true
		}

		if !hostnamesDone && len(epList.Hostnames) != 0 {
			seps.SelectedEndpoints.Hostnames = epList.Hostnames
			hostnamesDone = true
		}

		if ipv4Done && ipv6Done && hostnamesDone {
			break
		}
	}

	// compute local endpoints
	for _, info := range readyEndpoints {
		if info.Topology[hostNameLabel] == myNodeName {
			seps.LocalEndpoints.Add(info.AddressOrHostname)
		}
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
