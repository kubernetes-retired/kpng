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

package main

import (
	"bytes"
	"strconv"

	"k8s.io/klog"

	"m.cluseau.fr/kpng/pkg/api/localnetv1"
)

type dnatRule struct {
	Namespace   string
	Name        string
	Protocol    localnetv1.Protocol
	Ports       []*localnetv1.PortMapping
	EndpointIPs []string
}

// inlined write helpers
func writeInt(w *bytes.Buffer, v int) (int, error) {
	return w.WriteString(strconv.FormatInt(int64(v), 10))
}
func writeInt32(w *bytes.Buffer, v int32) (int, error) {
	return w.WriteString(strconv.FormatInt(int64(v), 10))
}
func writeUint64(w *bytes.Buffer, v uint64) (int, error) {
	return w.WriteString(strconv.FormatUint(v, 10))
}

func (d dnatRule) WriteTo(rule *bytes.Buffer, endpointsMap string, endpointsOffset uint64) {
	var protoMatch string
	switch d.Protocol {
	case localnetv1.Protocol_TCP:
		protoMatch = "tcp dport"
	case localnetv1.Protocol_UDP:
		protoMatch = "udp dport"
	case localnetv1.Protocol_SCTP:
		protoMatch = "sctp dport"
	default:
		klog.Errorf("unknown protocol: %v", d.Protocol)
		return
	}

	ports := make([]*localnetv1.PortMapping, 0, len(d.Ports))
	for _, port := range d.Ports {
		if port.Protocol != d.Protocol {
			continue
		}

		ports = append(ports, port)
	}

	if len(ports) == 0 {
		return
	}

	// printf is nice but take 50% on CPU time so... optimize!
	for _, port := range ports {
		rule.WriteString("  ")
		rule.WriteString(protoMatch)

		rule.WriteByte(' ')
		writeInt32(rule, port.Port)

		// handle reject case
		if len(d.EndpointIPs) == 0 {
			rule.WriteString(" counter reject\n")
			continue
		}

		// dnat case
		//fmt.Fprintf(out, "comment \"dnat for %s/%s port %d\" ", endpoints.Namespace, endpoints.Name, port.Port)
		if len(d.EndpointIPs) == 1 {
			// single destination
			rule.WriteString(" counter dnat to ")
			rule.Write([]byte(d.EndpointIPs[0]))

		} else {
			rule.WriteString(" counter dnat to numgen random mod ")
			writeInt(rule, len(d.EndpointIPs))
			rule.WriteString(" offset ")
			writeUint64(rule, endpointsOffset)
			rule.WriteString(" map @")
			rule.WriteString(endpointsMap)
		}

		if port.Port != port.TargetPort {
			rule.WriteByte(':')
			writeInt32(rule, port.TargetPort)
		}

		rule.WriteByte('\n')
	}

	return
}
