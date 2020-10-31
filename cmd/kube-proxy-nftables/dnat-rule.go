package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"k8s.io/klog"
)

type dnatRule struct {
	Namespace   string
	Name        string
	Familly     string
	ServiceIPs  []string
	Protocol    localnetv1.Protocol
	Ports       []*localnetv1.PortMapping
	EndpointIPs []string
}

func (d dnatRule) WriteTo(rule io.Writer) (n int64, err error) {
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

	printf := func(pattern string, values ...interface{}) {
		if err != nil {
			return
		}

		myN, myErr := fmt.Fprintf(rule, pattern, values...)
		n += int64(myN)
		if myErr != nil {
			err = myErr
		}
	}

	srcPorts := make([]string, 0)
	portMaps := make([]string, 0)
	var dstPort int32     // for the single port case
	portsIdentity := true // if every source port is mapped to the same target
	for _, port := range d.Ports {
		if port.Protocol != d.Protocol {
			continue
		}

		if portsIdentity && port.Port != port.TargetPort {
			portsIdentity = false
		}

		srcPorts = append(srcPorts, fmt.Sprintf("%d", port.Port))
		dstPort = port.TargetPort
		portMaps = append(portMaps, fmt.Sprintf("%d : %d", port.Port, port.TargetPort))
	}

	if len(srcPorts) == 0 {
		return
	}

	printf("  %s daddr ", d.Familly)
	if len(d.ServiceIPs) == 1 {
		printf("%s", d.ServiceIPs[0])
	} else {
		printf("{%s}", strings.Join(d.ServiceIPs, ", "))
	}

	if len(srcPorts) == 1 {
		printf(" %s %s", protoMatch, srcPorts[0])
	} else {
		printf(" %s {%s}", protoMatch, strings.Join(srcPorts, ", "))
	}

	dstIPs := make([]string, 0, len(d.EndpointIPs))
	dstMap := make([]string, 0, len(d.EndpointIPs))

	for idx, epIP := range d.EndpointIPs {
		dstIPs = append(dstIPs, epIP)
		dstMap = append(dstMap, fmt.Sprintf("%d : %s", idx, epIP))
	}

	//fmt.Fprintf(out, "comment \"dnat for %s/%s port %d\" ", endpoints.Namespace, endpoints.Name, port.Port)

	fmt.Fprint(rule, " ")
	if len(dstIPs) == 0 {
		printf("reject")
	} else {
		if len(dstIPs) == 1 {
			printf("dnat to %s", dstIPs[0])
		} else {
			printf("dnat to numgen random mod %d map {%s}", len(dstMap), strings.Join(dstMap, ", "))
		}

		if !portsIdentity {
			if len(portMaps) == 1 {
				printf(":%d", dstPort)
			} else {
				printf(":%s map { %s }", protoMatch, strings.Join(portMaps, ", "))
			}
		}
	}

	if !*skipComments {
		printf(" comment \"%s/%s %v\"", d.Namespace, d.Name, d.Protocol)
	}

	return
}
