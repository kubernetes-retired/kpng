//go:build windows
// +build windows

/*
Copyright 2018-2022 The Kubernetes Authors.

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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	//	"k8s.io/kubernetes/pkg/proxy"
	"net"
	"strconv"
)

// internal struct for windowsEndpoint information
type windowsEndpoint struct {
	ip              string
	port            uint16
	isLocal         bool
	macAddress      string
	hnsID           string
	refCount        *uint16
	providerAddress string
	hns             HostNetworkService

	// conditions
	ready       bool
	serving     bool
	terminating bool
}

// String is part of proxy.Endpoint interface.
func (ep *windowsEndpoint) String() string {
	return net.JoinHostPort(
		ep.ip,
		strconv.Itoa(int(ep.port)))
}

// GetIsLocal is part of proxy.Endpoint interface.
func (ep *windowsEndpoint) GetIsLocal() bool {
	return ep.isLocal
}

// IsReady returns true if an endpoint is ready and not terminating.
func (ep *windowsEndpoint) IsReady() bool {
	return ep.ready
}

// IsServing returns true if an endpoint is ready, regardless of it's terminating state.
func (ep *windowsEndpoint) IsServing() bool {
	return ep.serving
}

// IsTerminating returns true if an endpoint is terminating.
func (ep *windowsEndpoint) IsTerminating() bool {
	return ep.terminating
}

// GetZoneHint returns the zone hint for the endpoint.
func (ep *windowsEndpoint) GetZoneHints() sets.String {
	return sets.String{}
}

// IP returns just the IP part of the endpoint, it's a part of proxy.Endpoint interface.
func (ep *windowsEndpoint) IP() string {
	return ep.ip
}

// Port returns just the Port part of the endpoint.
func (ep *windowsEndpoint) Port() (int, error) {
	return int(ep.port), nil
}

// Equal is part of proxy.Endpoint interface.
func (ep *windowsEndpoint) Equal(other Endpoint) bool {
	return ep.String() == other.String() && ep.GetIsLocal() == other.GetIsLocal()
}

// GetNodeName returns the NodeName for this endpoint.
func (ep *windowsEndpoint) GetNodeName() string {
	return ""
}

// GetZone returns the Zone for this endpoint.
func (ep *windowsEndpoint) GetZone() string {
	return ""
}

// TODO Jay imp topology logic mahybe in baseEndpointInfo or converge it w baseEndpointInfo
func (ep *windowsEndpoint) GetTopology() map[string]string {
	return map[string]string{}
}

func (ep *windowsEndpoint) Cleanup() {
	klog.V(3).InfoS("Endpoint cleanup", "windowsEndpoint.Info", ep)
	if !ep.GetIsLocal() && ep.refCount != nil {
		*ep.refCount--

		// Remove the remote hns endpoint, if no service is referring it
		// Never delete a Local Endpoint. Local Endpoints are already created by other entities.
		// Remove only remote windowsEndpoint created by this service
		if *ep.refCount <= 0 && !ep.GetIsLocal() {
			klog.V(4).InfoS("Removing windowsEndpoint, since no one is referencing it", "endpoint", ep)
			err := ep.hns.deleteEndpoint(ep.hnsID)
			if err == nil {
				ep.hnsID = ""
			} else {
				klog.ErrorS(err, "Endpoint deletion failed", "ip", ep.IP())
			}
		}

		ep.refCount = nil
	}
}
