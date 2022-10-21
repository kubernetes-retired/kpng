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

package conntrack

import (
	"bytes"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	utilnet "k8s.io/utils/net"

	v1 "sigs.k8s.io/kpng/api/localnetv1"
)

var execer = exec.New()

func setupConntrack() {
	// TODO
}

func cleanupIPPortEntries(ipp IPPort) {
	parameters := parametersWithFamily(utilnet.IsIPv6String(ipp.DnatIP), "-D",
		"-p", protoStr(ipp.Protocol), "--dport", strconv.Itoa(int(ipp.Port)))
	if ipp.DnatIP != "node" {
		parameters = append(parameters, "--orig-dst", ipp.DnatIP)
	}

	klog.V(4).Infof("Clearing conntrack entries for (IP,Port) %v", parameters)
	output, err := runConntrack(parameters...)
	if err != nil {
		return
	}
	klog.V(4).Infof("Conntrack entries for (IP,Port) deleted: %s", string(output))
}

func cleanupFlowEntries(flow Flow) {
	if !IsClearConntrackNeeded(flow.Protocol) {
		return
	}

	// adapted & completed from k8s's pkg/util/conntrack

	parameters := parametersWithFamily(utilnet.IsIPv6String(flow.DnatIP), "-D",
		"-p", protoStr(flow.Protocol), "--dport", strconv.Itoa(int(flow.Port)),
		"--dst-nat", flow.EndpointIP)

	if flow.DnatIP != "node" {
		parameters = append(parameters, "--orig-dst", flow.DnatIP)
	}

	klog.V(4).Infof("Clearing conntrack entries %v", parameters)
	output, err := runConntrack(parameters...)
	if err != nil {
		return
	}
	klog.V(4).Infof("Conntrack entries deleted: %s", string(output))
}

func runConntrack(parameters ...string) (output []byte, err error) {
	conntrackPath, err := execer.LookPath("conntrack")
	if err != nil {
		klog.Errorf("error looking for path of conntrack: %v", err)
		return
	}
	output, err = execer.Command(conntrackPath, parameters...).CombinedOutput()
	if err != nil {
		if bytes.Contains(output, []byte(" 0 flow entries have been deleted")) {
			err = nil
		} else {
			klog.Errorf("conntrack command failed: %v: %v", parameters, err)
			if len(output) != 0 {
				klog.Errorf("conntrack command output: %s", string(output))
			}
		}
		return
	}
	return
}

// adapted from k8s's pkg/util/conntrack

func parametersWithFamily(isIPv6 bool, parameters ...string) []string {
	if isIPv6 {
		parameters = append(parameters, "-f", "ipv6")
	}
	return parameters
}

func protoStr(proto v1.Protocol) string {
	return strings.ToLower(proto.String())
}

// IsClearConntrackNeeded returns true if protocol requires conntrack cleanup for the stale connections
func IsClearConntrackNeeded(proto v1.Protocol) bool {
	return proto == v1.Protocol_UDP || proto == v1.Protocol_SCTP
}
