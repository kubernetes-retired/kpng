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
	"fmt"
	v1 "k8s.io/api/core/v1"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
)

const (
	kubeLoopBackIPSetComment = "Kubernetes endpoints dst ip:port, source ip for solving hairpin purpose"
	kubeLoopBackIPSet        = "KUBE-LOOP-BACK"

	kubeClusterIPSetComment = "Kubernetes service cluster ip + port for masquerade purpose"
	kubeClusterIPSet        = "KUBE-CLUSTER-IP"

	kubeExternalIPSetComment = "Kubernetes service external ip + port for masquerade and filter purpose"
	kubeExternalIPSet        = "KUBE-EXTERNAL-IP"

	kubeExternalIPLocalSetComment = "Kubernetes service external ip + port with externalTrafficPolicy=local"
	kubeExternalIPLocalSet        = "KUBE-EXTERNAL-IP-LOCAL"

	kubeLoadBalancerSetComment = "Kubernetes service lb portal"
	kubeLoadBalancerSet        = "KUBE-LOAD-BALANCER"

	kubeLoadBalancerLocalSetComment = "Kubernetes service load balancer ip + port with externalTrafficPolicy=local"
	kubeLoadBalancerLocalSet        = "KUBE-LOAD-BALANCER-LOCAL"

	kubeLoadbalancerFWSetComment = "Kubernetes service load balancer ip + port for load balancer with sourceRange"
	kubeLoadbalancerFWSet        = "KUBE-LOAD-BALANCER-FW"

	kubeLoadBalancerSourceIPSetComment = "Kubernetes service load balancer ip + port + source IP for packet filter purpose"
	kubeLoadBalancerSourceIPSet        = "KUBE-LOAD-BALANCER-SOURCE-IP"

	kubeLoadBalancerSourceCIDRSetComment = "Kubernetes service load balancer ip + port + source cidr for packet filter purpose"
	kubeLoadBalancerSourceCIDRSet        = "KUBE-LOAD-BALANCER-SOURCE-CIDR"

	kubeNodePortSetTCPComment = "Kubernetes nodeport TCP port for masquerade purpose"
	kubeNodePortSetTCP        = "KUBE-NODE-PORT-TCP"

	kubeNodePortLocalSetTCPComment = "Kubernetes nodeport TCP port with externalTrafficPolicy=local"
	kubeNodePortLocalSetTCP        = "KUBE-NODE-PORT-LOCAL-TCP"

	kubeNodePortSetUDPComment = "Kubernetes nodeport UDP port for masquerade purpose"
	kubeNodePortSetUDP        = "KUBE-NODE-PORT-UDP"

	kubeNodePortLocalSetUDPComment = "Kubernetes nodeport UDP port with externalTrafficPolicy=local"
	kubeNodePortLocalSetUDP        = "KUBE-NODE-PORT-LOCAL-UDP"

	kubeNodePortSetSCTPComment = "Kubernetes nodeport SCTP port for masquerade purpose with type 'hash ip:port'"
	kubeNodePortSetSCTP        = "KUBE-NODE-PORT-SCTP-HASH"

	kubeNodePortLocalSetSCTPComment = "Kubernetes nodeport SCTP port with externalTrafficPolicy=local with type 'hash ip:port'"
	kubeNodePortLocalSetSCTP        = "KUBE-NODE-PORT-LOCAL-SCTP-HASH"

	kubeHealthCheckNodePortSetComment = "Kubernetes health check node port"
	kubeHealthCheckNodePortSet        = "KUBE-HEALTH-CHECK-NODE-PORT"
)

// ipsetInfo is all ipset we needed in ipvs proxier
var ipsetInfo = []struct {
	name    string
	setType ipsetutil.Type
	comment string
}{
	{kubeLoopBackIPSet, ipsetutil.HashIPPortIP, kubeLoopBackIPSetComment},
	{kubeClusterIPSet, ipsetutil.HashIPPort, kubeClusterIPSetComment},
	{kubeExternalIPSet, ipsetutil.HashIPPort, kubeExternalIPSetComment},
	{kubeExternalIPLocalSet, ipsetutil.HashIPPort, kubeExternalIPLocalSetComment},
	{kubeLoadBalancerSet, ipsetutil.HashIPPort, kubeLoadBalancerSetComment},
	{kubeLoadbalancerFWSet, ipsetutil.HashIPPort, kubeLoadbalancerFWSetComment},
	{kubeLoadBalancerLocalSet, ipsetutil.HashIPPort, kubeLoadBalancerLocalSetComment},
	{kubeLoadBalancerSourceIPSet, ipsetutil.HashIPPortIP, kubeLoadBalancerSourceIPSetComment},
	{kubeLoadBalancerSourceCIDRSet, ipsetutil.HashIPPortNet, kubeLoadBalancerSourceCIDRSetComment},
	{kubeNodePortSetTCP, ipsetutil.BitmapPort, kubeNodePortSetTCPComment},
	{kubeNodePortLocalSetTCP, ipsetutil.BitmapPort, kubeNodePortLocalSetTCPComment},
	{kubeNodePortSetUDP, ipsetutil.BitmapPort, kubeNodePortSetUDPComment},
	{kubeNodePortLocalSetUDP, ipsetutil.BitmapPort, kubeNodePortLocalSetUDPComment},
	{kubeNodePortSetSCTP, ipsetutil.HashIPPort, kubeNodePortSetSCTPComment},
	{kubeNodePortLocalSetSCTP, ipsetutil.HashIPPort, kubeNodePortLocalSetSCTPComment},
	{kubeHealthCheckNodePortSet, ipsetutil.BitmapPort, kubeHealthCheckNodePortSetComment},
}

// IPSetVersioner can query the current ipset version.
type IPSetVersioner interface {
	// returns "X.Y"
	GetVersion() (string, error)
}

type IPSet struct {
	ipsetutil.IPSet
	// activeEntries is the current active entries of the ipset.
	newEntries sets.String
	// activeEntries is the current active entries of the ipset.
	deleteEntries sets.String
	// handle is the util ipset interface handle.
	handle ipsetutil.Interface

	refCountOfSvc int
}

// NewIPSet initialize a new IPSet struct
func newIPSet(handle ipsetutil.Interface, name string, setType ipsetutil.Type, ipFamily v1.IPFamily, comment string) *IPSet {
	hashFamily := ipsetutil.ProtocolFamilyIPV4
	if ipFamily == v1.IPv6Protocol {
		hashFamily = ipsetutil.ProtocolFamilyIPV6
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
				klog.InfoS("Ipset name truncated", "ipSetName", name, "truncatedName", name[:31])
				name = name[:31]
			}
		}
	}
	set := &IPSet{
		IPSet: ipsetutil.IPSet{
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


func (set *IPSet) isEmpty() bool {
	return len(set.newEntries.UnsortedList()) == 0
}

func (set *IPSet) isRefCountZero() bool {
	return set.refCountOfSvc == 0
}

func (set *IPSet) validateEntry(entry *ipsetutil.Entry) bool {
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
