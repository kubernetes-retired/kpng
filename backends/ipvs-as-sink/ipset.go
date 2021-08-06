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

package ipvssink

import (
	"k8s.io/apimachinery/pkg/util/sets"

	utilipset "sigs.k8s.io/kpng/backends/util/ipvs"

	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

const (
	kubeLoopBackIPSetComment = "Kubernetes endpoints dst ip:port, source ip for solving hairpin purpose"
	kubeLoopBackIPv4Set      = "KUBE-LOOP-BACK"
	kubeLoopBackIPv6Set      = "KUBE-6-LOOP-BACK"

	kubeClusterIPSetComment = "Kubernetes service cluster ip + port for masquerade purpose"
	kubeClusterIPv4Set      = "KUBE-CLUSTER-IP"
	kubeClusterIPv6Set      = "KUBE-6-CLUSTER-IP"

	kubeExternalIPSetComment = "Kubernetes service external ip + port for masquerade and filter purpose"
	kubeExternalIPv4Set      = "KUBE-EXTERNAL-IP"
	kubeExternalIPv6Set      = "KUBE-6-EXTERNAL-IP"

	kubeExternalIPLocalSetComment = "Kubernetes service external ip + port with externalTrafficPolicy=local"
	kubeExternalIPv4LocalSet      = "KUBE-EXTERNAL-IP-LOCAL"
	kubeExternalIPv6LocalSet      = "KUBE-6-EXTERNAL-IP-LOCAL"

	kubeLoadBalancerSetComment = "Kubernetes service lb portal"
	kubeLoadBalancerIPv4Set    = "KUBE-LOAD-BALANCER"
	kubeLoadBalancerIPv6Set    = "KUBE-6-LOAD-BALANCER"

	kubeLoadBalancerLocalSetComment = "Kubernetes service load balancer ip + port with externalTrafficPolicy=local"
	kubeLoadBalancerLocalIPv4Set    = "KUBE-LOAD-BALANCER-LOCAL"
	kubeLoadBalancerLocalIPv6Set    = "KUBE-6-LOAD-BALANCER-LOCAL"

	kubeLoadbalancerFWSetComment = "Kubernetes service load balancer ip + port for load balancer with sourceRange"
	kubeLoadbalancerFWIPv4Set    = "KUBE-LOAD-BALANCER-FW"
	kubeLoadbalancerFWIPv6Set    = "KUBE-6-LOAD-BALANCER-FW"

	kubeLoadBalancerSourceIPSetComment = "Kubernetes service load balancer ip + port + source IP for packet filter purpose"
	kubeLoadBalancerSourceIPv4Set      = "KUBE-LOAD-BALANCER-SOURCE-IP"
	kubeLoadBalancerSourceIPv6Set      = "KUBE-6-LOAD-BALANCER-SOURCE-IP"

	kubeLoadBalancerSourceCIDRSetComment = "Kubernetes service load balancer ip + port + source cidr for packet filter purpose"
	kubeLoadBalancerSourceCIDRIPv4Set    = "KUBE-LOAD-BALANCER-SOURCE-CIDR"
	kubeLoadBalancerSourceCIDRIPv6Set    = "KUBE-6-LOAD-BALANCER-SOURCE-CIDR"

	kubeNodePortSetTCPComment = "Kubernetes nodeport TCP port for masquerade purpose"
	kubeNodePortIPv4SetTCP    = "KUBE-NODE-PORT-TCP"
	kubeNodePortIPv6SetTCP    = "KUBE-6-NODE-PORT-TCP"

	kubeNodePortLocalSetTCPComment = "Kubernetes nodeport TCP port with externalTrafficPolicy=local"
	kubeNodePortLocalIPv4SetTCP    = "KUBE-NODE-PORT-LOCAL-TCP"
	kubeNodePortLocalIPv6SetTCP    = "KUBE-6-NODE-PORT-LOCAL-TCP"

	kubeNodePortSetUDPComment = "Kubernetes nodeport UDP port for masquerade purpose"
	kubeNodePortIPv4SetUDP    = "KUBE-NODE-PORT-UDP"
	kubeNodePortIPv6SetUDP    = "KUBE-6-NODE-PORT-UDP"

	kubeNodePortLocalSetUDPComment = "Kubernetes nodeport UDP port with externalTrafficPolicy=local"
	kubeNodePortLocalIPv4SetUDP    = "KUBE-NODE-PORT-LOCAL-UDP"
	kubeNodePortLocalIPv6SetUDP    = "KUBE-6-NODE-PORT-LOCAL-UDP"

	kubeNodePortSetSCTPComment = "Kubernetes nodeport SCTP port for masquerade purpose with type 'hash ip:port'"
	kubeNodePortIPv4SetSCTP    = "KUBE-NODE-PORT-SCTP-HASH"
	kubeNodePortIPv6SetSCTP    = "KUBE-6-NODE-PORT-SCTP-HASH"

	kubeNodePortLocalSetSCTPComment = "Kubernetes nodeport SCTP port with externalTrafficPolicy=local with type 'hash ip:port'"
	kubeNodePortLocalIPv4SetSCTP    = "KUBE-NODE-PORT-LOCAL-SCTP-HASH"
	kubeNodePortLocalIPv6SetSCTP    = "KUBE-6-NODE-PORT-LOCAL-SCTP-HASH"

	kubeHealthCheckNodePortSetComment = "Kubernetes health check node port"
	kubeHealthCheckNodePortIPv4Set    = "KUBE-HEALTH-CHECK-NODE-PORT"
	kubeHealthCheckNodePortIPv6Set    = "KUBE-6-HEALTH-CHECK-NODE-PORT"
)

// ipsetInfo is all ipset we needed in ipvs proxier
var ipsetInfo = []struct {
	name    string
	setType utilipset.Type
	comment string
}{
	{kubeLoopBackIPv4Set, utilipset.HashIPPortIP, kubeLoopBackIPSetComment},
	{kubeClusterIPv4Set, utilipset.HashIPPort, kubeClusterIPSetComment},
	{kubeExternalIPv4Set, utilipset.HashIPPort, kubeExternalIPSetComment},
	{kubeExternalIPv4LocalSet, utilipset.HashIPPort, kubeExternalIPLocalSetComment},
	{kubeLoadBalancerIPv4Set, utilipset.HashIPPort, kubeLoadBalancerSetComment},
	{kubeLoadbalancerFWIPv4Set, utilipset.HashIPPort, kubeLoadbalancerFWSetComment},
	{kubeLoadBalancerLocalIPv4Set, utilipset.HashIPPort, kubeLoadBalancerLocalSetComment},
	{kubeLoadBalancerSourceIPv4Set, utilipset.HashIPPortIP, kubeLoadBalancerSourceIPSetComment},
	{kubeLoadBalancerSourceCIDRIPv4Set, utilipset.HashIPPortNet, kubeLoadBalancerSourceCIDRSetComment},
	{kubeNodePortIPv4SetTCP, utilipset.BitmapPort, kubeNodePortSetTCPComment},
	{kubeNodePortLocalIPv4SetTCP, utilipset.BitmapPort, kubeNodePortLocalSetTCPComment},
	{kubeNodePortIPv4SetUDP, utilipset.BitmapPort, kubeNodePortSetUDPComment},
	{kubeNodePortLocalIPv4SetUDP, utilipset.BitmapPort, kubeNodePortLocalSetUDPComment},
	{kubeNodePortIPv4SetSCTP, utilipset.HashIPPort, kubeNodePortSetSCTPComment},
	{kubeNodePortLocalIPv4SetSCTP, utilipset.HashIPPort, kubeNodePortLocalSetSCTPComment},
	{kubeHealthCheckNodePortIPv4Set, utilipset.BitmapPort, kubeHealthCheckNodePortSetComment},
}

// IPSetVersioner can query the current ipset version.
type IPSetVersioner interface {
	// returns "X.Y"
	GetVersion() (string, error)
}

type IPSet struct {
	utilipset.IPSet
	// activeEntries is the current active entries of the ipset.
	newEntries sets.String
	// activeEntries is the current active entries of the ipset.
	deleteEntries sets.String
	// handle is the util ipset interface handle.
	handle utilipset.Interface
}

// NewIPSet initialize a new IPSet struct
func newIPv4Set(handle utilipset.Interface, name string, setType utilipset.Type, comment string) *IPSet {
	hashFamily := utilipset.ProtocolFamilyIPV4
	set := &IPSet{
		IPSet: utilipset.IPSet{
			Name:       name,
			SetType:    setType,
			HashFamily: hashFamily,
			Comment:    comment,
		},
		newEntries:    sets.NewString(),
		deleteEntries: sets.NewString(),
		handle:        handle,
	}
	return set
}

// NewIPSet initialize a new IPSet struct
func newIPv6Set(handle utilipset.Interface, name string, setType utilipset.Type, comment string) *IPSet {
	hashFamily := utilipset.ProtocolFamilyIPV6
	// In dual-stack both ipv4 and ipv6 ipset's can co-exist. To
	// ensure unique names the prefix for ipv6 is changed from
	// "KUBE-" to "KUBE-6-". The "KUBE-" prefix is kept for
	// backward compatibility. The maximum name length of an ipset
	// is 31 characters which must be taken into account.  The
	// ipv4 names are not altered to minimize the risk for
	// problems on upgrades.
	if strings.HasPrefix(name, "KUBE-") {
		name = strings.Replace(name, "KUBE-", "KUBE-6-", 1)
		if len(name) > 31 {
			klog.Warningf("ipset name truncated; [%s] -> [%s]", name, name[:31])
			name = name[:31]
		}
	}
	set := &IPSet{
		IPSet: utilipset.IPSet{
			Name:       name,
			SetType:    setType,
			HashFamily: hashFamily,
			Comment:    comment,
		},
		newEntries:    sets.NewString(),
		deleteEntries: sets.NewString(),
		handle:        handle,
	}
	return set
}

func (set *IPSet) validateEntry(entry *utilipset.Entry) bool {
	return entry.Validate(&set.IPSet)
}

func (set *IPSet) getComment() string {
	return fmt.Sprintf("\"%s\"", set.Comment)
}

func (set *IPSet) resetEntries() {
	set.newEntries = sets.NewString()
	set.deleteEntries = sets.NewString()
}

func (set *IPSet) syncIPSetEntries() {
	// Create entries
	for _, entry := range set.newEntries.List() {
		if err := set.handle.AddEntry(entry, &set.IPSet, true); err != nil {
			klog.Errorf("Failed to add entry %v into ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully add entry: %v into ip set: %s", entry, set.Name)
		}
	}

	// Delete entries
	for _, entry := range set.deleteEntries.List() {
		if err := set.handle.DelEntry(entry, set.Name); err != nil {
			klog.Errorf("Failed to delete entry: %v from ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully deleted entry: %v to ip set: %s", entry, set.Name)
		}
	}

}

func ensureIPSet(set *IPSet) error {
	if err := set.handle.CreateSet(&set.IPSet, true); err != nil {
		klog.Errorf("Failed to ensure ip set %v exist, error: %v", set, err)
		return err
	}
	return nil
}
