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
	"bytes"
	"fmt"

	"k8s.io/klog/v2"

	iptablesutil "sigs.k8s.io/kpng/backends/iptables/util"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
)

const (
	// kubeServicesChain is the services portal chain
	kubeServicesChain iptablesutil.Chain = "KUBE-SERVICES"

	// KubeFireWallChain is the kubernetes firewall chain.
	KubeFireWallChain iptablesutil.Chain = "KUBE-FIREWALL"

	// kubePostroutingChain is the kubernetes postrouting chain
	kubePostroutingChain iptablesutil.Chain = "KUBE-POSTROUTING"

	// KubeMarkMasqChain is the mark-for-masquerade chain
	KubeMarkMasqChain iptablesutil.Chain = "KUBE-MARK-MASQ"

	// KubeNodePortChain is the kubernetes node port chain
	KubeNodePortChain iptablesutil.Chain = "KUBE-NODE-PORT"

	// KubeMarkDropChain is the mark-for-drop chain
	KubeMarkDropChain iptablesutil.Chain = "KUBE-MARK-DROP"

	// KubeForwardChain is the kubernetes forward chain
	KubeForwardChain iptablesutil.Chain = "KUBE-FORWARD"

	// KubeLoadBalancerChain is the kubernetes chain for loadbalancer type service
	KubeLoadBalancerChain iptablesutil.Chain = "KUBE-LOAD-BALANCER"
)

var iptablesEnsureChains = []struct {
	table iptablesutil.Table
	chain iptablesutil.Chain
}{
	{iptablesutil.TableNAT, KubeMarkDropChain},
}

var iptablesChains = []struct {
	table iptablesutil.Table
	chain iptablesutil.Chain
}{
	{iptablesutil.TableNAT, kubeServicesChain},
	{iptablesutil.TableNAT, kubePostroutingChain},
	{iptablesutil.TableNAT, KubeFireWallChain},
	{iptablesutil.TableNAT, KubeNodePortChain},
	{iptablesutil.TableNAT, KubeLoadBalancerChain},
	{iptablesutil.TableNAT, KubeMarkMasqChain},
	{iptablesutil.TableFilter, KubeForwardChain},
	{iptablesutil.TableFilter, KubeNodePortChain},
}

// iptablesJumpChain is tables of iptables chains that ipvs proxier used to install iptables or cleanup iptables.
// `to` is the iptables chain we want to operate.
// `from` is the source iptables chain
var iptablesJumpChain = []struct {
	table   iptablesutil.Table
	from    iptablesutil.Chain
	to      iptablesutil.Chain
	comment string
}{
	{iptablesutil.TableNAT, iptablesutil.ChainOutput, kubeServicesChain, "kubernetes service portals"},
	{iptablesutil.TableNAT, iptablesutil.ChainPrerouting, kubeServicesChain, "kubernetes service portals"},
	{iptablesutil.TableNAT, iptablesutil.ChainPostrouting, kubePostroutingChain, "kubernetes postrouting rules"},
	{iptablesutil.TableFilter, iptablesutil.ChainForward, KubeForwardChain, "kubernetes forwarding rules"},
	{iptablesutil.TableFilter, iptablesutil.ChainInput, KubeNodePortChain, "kubernetes health check rules"},
}

// ipsetWithIptablesChain is the ipsets list with iptables source chain and the chain jump to
// `iptables -t nat -A <from> -m set --match-set <name> <matchType> -j <to>`
// example: iptables -t nat -A KUBE-SERVICES -m set --match-set KUBE-NODE-PORT-TCP dst -j KUBE-NODE-PORT
// ipsets with other match rules will be created Individually.
// Note: kubeNodePortLocalSetTCP must be prior to kubeNodePortSetTCP, the same for UDP.
var ipsetWithIptablesChain = []struct {
	name          string
	from          string
	to            string
	matchType     string
	protocolMatch string
}{
	{kubeLoopBackIPSet, string(kubePostroutingChain), "MASQUERADE", "dst,dst,src", ""},
	{kubeLoadBalancerSet, string(kubeServicesChain), string(KubeLoadBalancerChain), "dst,dst", ""},
	{kubeLoadbalancerFWSet, string(KubeLoadBalancerChain), string(KubeFireWallChain), "dst,dst", ""},
	{kubeLoadBalancerSourceCIDRSet, string(KubeFireWallChain), "RETURN", "dst,dst,src", ""},
	{kubeLoadBalancerSourceIPSet, string(KubeFireWallChain), "RETURN", "dst,dst,src", ""},
	{kubeLoadBalancerLocalSet, string(KubeLoadBalancerChain), "RETURN", "dst,dst", ""},
	{kubeNodePortLocalSetTCP, string(KubeNodePortChain), "RETURN", "dst", ipsetutil.ProtocolTCP},
	{kubeNodePortSetTCP, string(KubeNodePortChain), string(KubeMarkMasqChain), "dst", ipsetutil.ProtocolTCP},
	{kubeNodePortLocalSetUDP, string(KubeNodePortChain), "RETURN", "dst", ipsetutil.ProtocolUDP},
	{kubeNodePortSetUDP, string(KubeNodePortChain), string(KubeMarkMasqChain), "dst", ipsetutil.ProtocolUDP},
	{kubeNodePortLocalSetSCTP, string(KubeNodePortChain), "RETURN", "dst,dst", ipsetutil.ProtocolSCTP},
	{kubeNodePortSetSCTP, string(KubeNodePortChain), string(KubeMarkMasqChain), "dst,dst", ipsetutil.ProtocolSCTP},
}

// createAndLinkKubeChain create all kube chains that ipvs proxier need and write basic link.
func (p *proxier) createAndLinkKubeChain() {
	existingFilterChains := p.getExistingChains(p.filterChainsData, iptablesutil.TableFilter)
	existingNATChains := p.getExistingChains(p.iptablesData, iptablesutil.TableNAT)

	// ensure KUBE-MARK-DROP chain exist but do not change any rules
	for _, ch := range iptablesEnsureChains {
		if _, err := p.iptables.EnsureChain(ch.table, ch.chain); err != nil {
			klog.ErrorS(err, "Failed to ensure chain exists", "table", ch.table, "chain", ch.chain)
			return
		}
	}

	// Make sure we keep stats for the top-level chains
	for _, ch := range iptablesChains {
		if _, err := p.iptables.EnsureChain(ch.table, ch.chain); err != nil {
			klog.ErrorS(err, "Failed to ensure chain exists", "table", ch.table, "chain", ch.chain)
			return
		}
		if ch.table == iptablesutil.TableNAT {
			if chain, ok := existingNATChains[ch.chain]; ok {
				p.natChains.WriteBytes(chain)
			} else {
				p.natChains.Write(iptablesutil.MakeChainLine(ch.chain))
			}
		} else {
			if chain, ok := existingFilterChains[ch.chain]; ok {
				p.filterChains.WriteBytes(chain)
			} else {
				p.filterChains.Write(iptablesutil.MakeChainLine(ch.chain))
			}
		}
	}

	for _, jc := range iptablesJumpChain {
		args := []string{"-m", "comment", "--comment", jc.comment, "-j", string(jc.to)}
		if _, err := p.iptables.EnsureRule(iptablesutil.Prepend, jc.table, jc.from, args...); err != nil {
			klog.ErrorS(err, "Failed to ensure chain jumps", "table", jc.table, "srcChain", jc.from, "dstChain", jc.to)
		}
	}
}

// getExistingChains get iptables-save output so we can check for existing chains and rules.
// This will be a map of chain name to chain with rules as stored in iptables-save/iptables-restore
// Result may SHARE memory with contents of buffer.
func (p *proxier) getExistingChains(buffer *bytes.Buffer, table iptablesutil.Table) map[iptablesutil.Chain][]byte {
	buffer.Reset()
	err := p.iptables.SaveInto(table, buffer)
	if err != nil { // if we failed to get any rules
		klog.ErrorS(err, "Failed to execute iptables-save, syncing all rules")
	} else { // otherwise parse the output
		return iptablesutil.GetChainLines(table, buffer.Bytes())
	}
	return nil
}

// writeIptablesRules write all iptables rules to proxier.natRules or proxier.FilterRules that ipvs proxier needed
// according to proxier.ipsetList information and the ipset match relationship that `ipsetWithIptablesChain` specified.
// some ipset(kubeClusterIPSet for example) have particular match rules and iptables jump relation should be sync separately.
func (p *proxier) writeIptablesRules() {
	// We are creating those slices ones here to avoid memory reallocations
	// in every loop. Note that reuse the memory, instead of doing:
	//   slice = <some new slice>
	// you should always do one of the below:
	//   slice = slice[:0] // and then append to it
	//   slice = append(slice[:0], ...)
	// To avoid growing this slice, we arbitrarily set its size to 64,
	// there is never more than that many arguments for a single line.
	// Note that even if we go over 64, it will still be correct - it
	// is just for efficiency, not correctness.
	args := make([]string, 64)

	for _, set := range ipsetWithIptablesChain {
		if !p.ipsetList[set.name].isRefCountZero() {
			args = append(args[:0], "-A", set.from)
			if set.protocolMatch != "" {
				args = append(args, "-p", set.protocolMatch)
			}
			args = append(args,
				"-m", "comment", "--comment", p.ipsetList[set.name].getComment(),
				"-m", "set", "--match-set", p.ipsetList[set.name].Name,
				set.matchType,
			)
			p.natRules.Write(args, "-j", set.to)
		}
	}


	if !p.ipsetList[kubeClusterIPSet].isRefCountZero() {
		args = append(args[:0],
			"-A", string(kubeServicesChain),
			"-m", "comment", "--comment", p.ipsetList[kubeClusterIPSet].getComment(),
			"-m", "set", "--match-set", p.ipsetList[kubeClusterIPSet].Name,
		)
		if p.masqueradeAll {
			p.natRules.Write(args, "dst,dst", "-j", string(KubeMarkMasqChain))
			//TODO: localDetector code needs to be added later
			//} else if p.localDetector.IsImplemented() {
			//	// This masquerades off-cluster traffic to a service VIP.  The idea
			//	// is that you can establish a static route for your Service range,
			//	// routing to any node, and that node will bridge into the Service
			//	// for you.  Since that might bounce off-node, we masquerade here.
			//	// If/when we support "Local" policy for VIPs, we should update this.
			//	p.natRules.Write(p.localDetector.JumpIfNotLocal(append(args, "dst,dst"), string(KubeMarkMasqChain)))
			//} else {
		} else {
			// Masquerade all OUTPUT traffic coming from a service ip.
			// The kube dummy interface has all service VIPs assigned which
			// results in the service VIP being picked as the source IP to reach
			// a VIP. This leads to a connection from VIP:<random port> to
			// VIP:<service port>.
			// Always masquerading OUTPUT (node-originating) traffic with a VIP
			// source ip and service port destination fixes the outgoing connections.
			p.natRules.Write(args, "src,dst", "-j", string(KubeMarkMasqChain))
		}
	}

	// externalIPRules adds iptables rules applies to Service ExternalIPs
	externalIPRules := func(args []string) {
		// Allow traffic for external IPs that does not come from a bridge (i.e. not from a container)
		// nor from a local process to be forwarded to the service.
		// This rule roughly translates to "all traffic from off-machine".
		// This is imperfect in the face of network plugins that might not use a bridge, but we can revisit that later.
		externalTrafficOnlyArgs := append(args,
			"-m", "physdev", "!", "--physdev-is-in",
			"-m", "addrtype", "!", "--src-type", "LOCAL")
		p.natRules.Write(externalTrafficOnlyArgs, "-j", "ACCEPT")
		dstLocalOnlyArgs := append(args, "-m", "addrtype", "--dst-type", "LOCAL")
		// Allow traffic bound for external IPs that happen to be recognized as local IPs to stay local.
		// This covers cases like GCE load-balancers which get added to the local routing table.
		p.natRules.Write(dstLocalOnlyArgs, "-j", "ACCEPT")
	}

	if !p.ipsetList[kubeExternalIPSet].isRefCountZero() {
		// Build masquerade rules for packets to external IPs.
		args = append(args[:0],
			"-A", string(kubeServicesChain),
			"-m", "comment", "--comment", p.ipsetList[kubeExternalIPSet].getComment(),
			"-m", "set", "--match-set", p.ipsetList[kubeExternalIPSet].Name,
			"dst,dst",
		)
		p.natRules.Write(args, "-j", string(KubeMarkMasqChain))
		externalIPRules(args)
	}

	if !p.ipsetList[kubeExternalIPLocalSet].isRefCountZero() {
		args = append(args[:0],
			"-A", string(kubeServicesChain),
			"-m", "comment", "--comment", p.ipsetList[kubeExternalIPLocalSet].getComment(),
			"-m", "set", "--match-set", p.ipsetList[kubeExternalIPLocalSet].Name,
			"dst,dst",
		)
		externalIPRules(args)
	}

	// -A KUBE-SERVICES  -m addrtype  --dst-type LOCAL -j KUBE-NODE-PORT
	args = append(args[:0],
		"-A", string(kubeServicesChain),
		"-m", "addrtype", "--dst-type", "LOCAL",
	)
	p.natRules.Write(args, "-j", string(KubeNodePortChain))

	// mark drop for KUBE-LOAD-BALANCER
	p.natRules.Write(
		"-A", string(KubeLoadBalancerChain),
		"-j", string(KubeMarkMasqChain),
	)
	// mark drop for KUBE-FIRE-WALL
	p.natRules.Write(
		"-A", string(KubeFireWallChain),
		"-j", string(KubeMarkDropChain),
	)

	// Accept all traffic with destination of ipvs virtual service, in case other iptables rules
	// block the traffic, that may result in ipvs rules invalid.
	// Those rules must be in the end of KUBE-SERVICE chain
	p.acceptIPVSTraffic()

	// If the masqueradeMark has been added then we want to forward that same
	// traffic, this allows NodePort traffic to be forwarded even if the default
	// FORWARD policy is not accept.
	p.filterRules.Write(
		"-A", string(KubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding rules"`,
		"-m", "mark", "--mark", fmt.Sprintf("%s/%s", p.masqueradeMark, p.masqueradeMark),
		"-j", "ACCEPT",
	)

	// The following rule ensures the traffic after the initial packet accepted
	// by the "kubernetes forwarding rules" rule above will be accepted.
	p.filterRules.Write(
		"-A", string(KubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding conntrack rule"`,
		"-m", "conntrack",
		"--ctstate", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	)

	// Add rule to accept traffic towards health check node port
	p.filterRules.Write(
		"-A", string(KubeNodePortChain),
		"-m", "comment", "--comment", p.ipsetList[kubeHealthCheckNodePortSet].getComment(),
		"-m", "set", "--match-set", p.ipsetList[kubeHealthCheckNodePortSet].Name, "dst",
		"-j", "ACCEPT",
	)

	// Install the kubernetes-specific postrouting rules. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	// NB: THIS MUST MATCH the corresponding code in the kubelet
	p.natRules.Write(
		"-A", string(kubePostroutingChain),
		"-m", "mark", "!", "--mark", fmt.Sprintf("%s/%s", p.masqueradeMark, p.masqueradeMark),
		"-j", "RETURN",
	)

	// Clear the mark to avoid re-masquerading if the packet re-traverses the network stack.
	p.natRules.Write(
		"-A", string(kubePostroutingChain),
		// XOR proxier.masqueradeMark to unset it
		"-j", "MARK", "--xor-mark", p.masqueradeMark,
	)

	masqRule := []string{
		"-A", string(kubePostroutingChain),
		"-m", "comment", "--comment", `"kubernetes service traffic requiring SNAT"`,
		"-j", "MASQUERADE",
	}
	if p.iptables.HasRandomFully() {
		masqRule = append(masqRule, "--random-fully")
	}
	p.natRules.Write(masqRule)

	// Install the kubernetes-specific masquerade mark rule. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	p.natRules.Write(
		"-A", string(KubeMarkMasqChain),
		"-j", "MARK", "--or-mark", p.masqueradeMark,
	)

	// Write the end-of-table markers.
	p.filterRules.Write("COMMIT")
	p.natRules.Write("COMMIT")
}

func (p *proxier) acceptIPVSTraffic() {
	sets := []string{kubeClusterIPSet, kubeLoadBalancerSet}
	for _, set := range sets {
		var matchType string
		if !p.ipsetList[set].isRefCountZero() {
			switch p.ipsetList[set].SetType {
			case ipsetutil.BitmapPort:
				matchType = "dst"
			default:
				matchType = "dst,dst"
			}
			p.natRules.Write(
				"-A", string(kubeServicesChain),
				"-m", "set", "--match-set", p.ipsetList[set].Name, matchType,
				"-j", "ACCEPT",
			)
		}
	}
}

func (p *proxier) syncIPTableRules() {
	// Reset all buffers used later.
	// This is to avoid memory reallocations and thus improve performance.
	p.natChains.Reset()
	p.natRules.Reset()
	p.filterChains.Reset()
	p.filterRules.Reset()

	// Write table headers.
	p.filterChains.Write("*filter")
	p.natChains.Write("*nat")

	p.createAndLinkKubeChain()

	// Tail call iptables rules for ipset, make sure only call iptables once
	// in a single loop per ip set.
	p.writeIptablesRules()

	// Sync iptables rules.
	// NOTE: NoFlushTables is used so we don't flush non-kubernetes chains in the table.
	p.iptablesData.Reset()
	p.iptablesData.Write(p.natChains.Bytes())
	p.iptablesData.Write(p.natRules.Bytes())
	p.iptablesData.Write(p.filterChains.Bytes())
	p.iptablesData.Write(p.filterRules.Bytes())

	klog.V(5).InfoS("Restoring iptables", "rules", string(p.iptablesData.Bytes()))
	err := p.iptables.RestoreAll(p.iptablesData.Bytes(), iptablesutil.NoFlushTables, iptablesutil.RestoreCounters)
	if err != nil {
		klog.ErrorS(err, "Failed to execute iptables-restore", "rules", string(p.iptablesData.Bytes()))
		return
	}
}