package endpoints

import (
	"net"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

var (
	_true  = true
	_false = false
)

func computeServiceEndpoints(src correlationSource) (eps *localnetv1.ServiceEndpoints) {
	if src.Service == nil {
		return
	}

	svcSpec := src.Service.Spec

	eps = &localnetv1.ServiceEndpoints{
		Namespace: src.Service.Namespace,
		Name:      src.Service.Name,
		Type:      string(svcSpec.Type),
		IPs: &localnetv1.ServiceIPs{
			ClusterIP:   svcSpec.ClusterIP,
			ExternalIPs: svcSpec.ExternalIPs,
		},
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

	eps.Ports = ports

	// normalize to EndpointSlice
	slices := make([]*discovery.EndpointSlice, 0)

	// TODO aggregate slices
	slices = append(slices, src.Slices...)

	if src.Endpoints != nil {
		// backward compat: convert Endpoints to []EndpointSlice

		for _, subset := range src.Endpoints.Subsets {
			// ports
			ports := make([]discovery.EndpointPort, len(subset.Ports))

			for idx, port := range subset.Ports {
				ports[idx] = discovery.EndpointPort{
					Name:     &port.Name,
					Protocol: &port.Protocol,
					Port:     &port.Port,
					// XXX and AppProtocol?
				}
			}

			// endpoints
			endpoints := make([]discovery.Endpoint, 0, len(subset.Addresses)+len(subset.NotReadyAddresses))
			for _, addr := range subset.Addresses {
				endpoints = append(endpoints, addrToEndpoint(addr, &_true))
			}
			for _, addr := range subset.NotReadyAddresses {
				endpoints = append(endpoints, addrToEndpoint(addr, &_false))
			}

			slice := &discovery.EndpointSlice{
				Ports:       ports,
				AddressType: discovery.AddressTypeIP, // FIXME
				Endpoints:   endpoints,
			}

			slices = append(slices, slice)
		}
	}

	// AllIPs
	allIPsV4 := sets.NewString()
	allIPsV6 := sets.NewString()

	for _, slice := range slices {
		for _, sliceEP := range slice.Endpoints {
			for _, addr := range sliceEP.Addresses {
				addrInsertByType(addr, slice.AddressType, allIPsV4, allIPsV6)
			}
		}
	}

	eps.AllIPsV4 = allIPsV4.List()
	eps.AllIPsV6 = allIPsV6.List()

	// TODO filters (topology etc)

	// endpoints
	endpoints := make([]*localnetv1.EndpointsSubset, 0)

	for _, slice := range slices {
		ports := make([]*localnetv1.Port, len(slice.Ports))
		for idx, port := range slice.Ports {
			p := &localnetv1.Port{}

			if port.Name != nil {
				p.Name = *port.Name
			}
			if port.Protocol != nil {
				p.Protocol = localnetv1.ParseProtocol(string(*port.Protocol))
			}
			if port.Port != nil {
				p.Port = *port.Port
			}

			ports[idx] = p
		}

		ipsv4 := sets.NewString()
		ipsv6 := sets.NewString()

		for _, sliceEP := range slice.Endpoints {
			for _, addr := range sliceEP.Addresses {
				addrInsertByType(addr, slice.AddressType, ipsv4, ipsv6)
			}
		}

		endpoints = append(endpoints, &localnetv1.EndpointsSubset{
			Ports: ports,
			IPsV4: ipsv4.List(),
			IPsV6: ipsv6.List(),
		})
	}

	eps.Subsets = endpoints

	return
}

func addrInsertByType(addr string, addrType discovery.AddressType, v4set, v6set sets.String) {
	switch addrType {
	case discovery.AddressTypeIPv4:
		v4set.Insert(addr)

	case discovery.AddressTypeIPv6:
		v6set.Insert(addr)

	case discovery.AddressTypeIP:
		if net.ParseIP(addr).To4() == nil {
			v6set.Insert(addr)
		} else {
			v4set.Insert(addr)
		}
	}
}

func addrToEndpoint(addr v1.EndpointAddress, ready *bool) (ep discovery.Endpoint) {
	ep = discovery.Endpoint{
		Addresses:  []string{addr.IP},
		Conditions: discovery.EndpointConditions{Ready: ready},
		// TODO Topology
	}

	if addr.Hostname != "" {
		ep.Hostname = &addr.Hostname
	}

	return
}
