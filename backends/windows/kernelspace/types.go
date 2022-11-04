//go:build windows
// +build windows

/*
Copyright 2015 The Kubernetes Authors.

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

package kernelspace

import (
	"fmt"
	"net"

	localv1 "sigs.k8s.io/kpng/api/localv1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ServicePortName carries a namespace + name + portname.  This is the unique
// identifier for a load-balanced service.
type ServicePortName struct {
	types.NamespacedName
	Port     string
	Protocol localv1.Protocol
}

func (spn ServicePortName) String() string {
	return fmt.Sprintf("%s%s", spn.NamespacedName.String(), fmtPortName(spn.Port))
}

func fmtPortName(in string) string {
	if in == "" {
		return ""
	}
	return fmt.Sprintf(":%s", in)
}

// ServicePort is an interface which abstracts information about a service.
type ServicePort interface {
	// String returns service string.  An example format can be: `IP:Port/Protocol`.
	String() string
	// GetClusterIP returns service cluster IP in net.IP format.
	ClusterIP() net.IP
	// GetPort returns service port if present. If return 0 means not present.
	Port() int
	// GetSessionAffinityType returns service session affinity type
	SessionAffinity() SessionAffinity
	// ExternalIPStrings returns service ExternalIPs as a string array.
	ExternalIPStrings() []string
	// LoadBalancerIPStrings returns service LoadBalancerIPs as a string array.
	LoadBalancerIPStrings() []string
	// GetProtocol returns service protocol.
	//	Protocol() kpng.Protocol
	Protocol() v1.Protocol

	// LoadBalancerSourceRanges returns service LoadBalancerSourceRanges if present empty array if not
	LoadBalancerSourceRanges() []string
	// GetHealthCheckNodePort returns service health check node port if present.  If return 0, it means not present.
	HealthCheckNodePort() int
	// GetNodePort returns a service Node port if present. If return 0, it means not present.
	NodePort() int
	// NodeLocalExternal returns if a service has only node local windowsEndpoint for external traffic.
	NodeLocalExternal() bool
	// NodeLocalInternal returns if a service has only node local windowsEndpoint for internal traffic.
	NodeLocalInternal() bool
	// InternalTrafficPolicy returns service InternalTrafficPolicy
	InternalTrafficPolicy() *v1.ServiceInternalTrafficPolicyType
	// HintsAnnotation returns the value of the v1.AnnotationTopologyAwareHints annotation.
	HintsAnnotation() string
}

// Endpoint in an interface which abstracts information about an endpoint.
// TODO: Rename functions to be consistent with ServicePort.
type Endpoint interface {
	// String returns endpoint string.  An example format can be: `IP:Port`.
	// We take the returned value as ServiceEndpoint.Endpoint.
	String() string
	// GetIsLocal returns true if the endpoint is running in same host as kube-proxy, otherwise returns false.
	GetIsLocal() bool
	// IsReady returns true if an endpoint is ready and not terminating.
	// This is only set when watching EndpointSlices. If using Endpoints, this is always
	// true since only ready windowsEndpoint are read from Endpoints.
	IsReady() bool
	// IsServing returns true if an endpoint is ready. It does not account
	// for terminating state.
	// This is only set when watching EndpointSlices. If using Endpoints, this is always
	// true since only ready windowsEndpoint are read from Endpoints.
	IsServing() bool
	// IsTerminating retruns true if an endpoint is terminating. For pods,
	// that is any pod with a deletion timestamp.
	// This is only set when watching EndpointSlices. If using Endpoints, this is always
	// false since terminating windowsEndpoint are always excluded from Endpoints.
	IsTerminating() bool
	// GetTopology returns the topology information of the endpoint.
	GetTopology() map[string]string
	// GetZoneHints returns the zone hint for the endpoint. This is based on
	// endpoint.hints.forZones[0].name in the EndpointSlice API.
	GetZoneHints() sets.String
	// IP returns IP part of the endpoint.
	IP() string
	// Port returns the Port part of the endpoint.
	Port() (int, error)
	// Equal checks if two windowsEndpoint are equal.
	Equal(Endpoint) bool
}

// ServiceEndpoint is used to identify a service and one of its endpoint pair.
type ServiceEndpoint struct {
	Endpoint        string
	ServicePortName ServicePortName
}
