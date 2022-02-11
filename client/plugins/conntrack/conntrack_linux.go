package conntrack

import (
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

func cleanupPotentialConflicts(flow Flow) {
	origin := flow.DnatIP
	parameters := parametersWithFamily(utilnet.IsIPv6String(origin), "-D",
		"--orig-dst", origin,
		"-p", protoStr(flow.Protocol),
		"--sport", strconv.Itoa(int(flow.Port)))

	klog.V(4).Infof("Clearing potential conflict conntrack entries %v", parameters)
	output, err := runConntrack(parameters...)
	if err != nil {
		klog.Errorf("conntrack command returned: %q, error message: %s", string(output), err)
		return
	}
	klog.V(4).Infof("Conntrack potential conflict entries deleted %s", string(output))
}

func cleanupFlowEntries(flow Flow) {
	if !IsClearConntrackNeeded(flow.Protocol) {
		return
	}

	origin := flow.DnatIP
	dest := flow.EndpointIP

	// adapted & completed from k8s's pkg/util/conntrack

	parameters := parametersWithFamily(utilnet.IsIPv6String(origin), "-D",
		"--orig-dst", origin, "--dst-nat", dest,
		"-p", protoStr(flow.Protocol),
		"--sport", strconv.Itoa(int(flow.Port)), "--dport", strconv.Itoa(int(flow.TargetPort)))

	klog.V(4).Infof("Clearing conntrack entries %v", parameters)
	output, err := runConntrack(parameters...)
	if err != nil {
		klog.Errorf("conntrack command returned: %q, error message: %s", string(output), err)
		return
	}
	klog.V(4).Infof("Conntrack entries deleted %s", string(output))
}

func runConntrack(parameters ...string) (output []byte, err error) {
	conntrackPath, err := execer.LookPath("conntrack")
	if err != nil {
		klog.Errorf("error looking for path of conntrack: %v", err)
		return
	}
	output, err = execer.Command(conntrackPath, parameters...).CombinedOutput()
	return
}

// adapted from k8s's pkg/util/conntrack

// NoConnectionToDelete is the error string returned by conntrack when no matching connections are found
const NoConnectionToDelete = "0 flow entries have been deleted"

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
