package iptables

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/backends/iptables/util"

	utilnet "k8s.io/utils/net"
)

var (
	onlyOutput    bool
	masqueradeAll bool
)

func BindFlags(flags *pflag.FlagSet) {
	flag.BoolVar(&onlyOutput, "only-output", false, "Only output the ipvsadm-restore file instead of calling ipvsadm-restore")
	flag.BoolVar(&masqueradeAll, "masquerade-all", false, "Set this flag to set the masq rule for all traffic")

}

type iptables struct {
	mu         sync.Mutex        // protects the following fields
	nodeLabels map[string]string //TODO: looks like can be removed as kpng controller shoujld do the work

	// endpointsSynced, endpointSlicesSynced, and servicesSynced are set to true
	// when corresponding objects are synced after startup. This is used to avoid
	// updating iptables with some partial data after kube-proxy restart.
	endpointsSynced      bool
	endpointSlicesSynced bool
	servicesSynced       bool
	initialized          int32
	syncPeriod           time.Duration

	// These are effectively const and do not need the mutex to be held.
	masqueradeAll  bool
	masqueradeMark string

	nodeIP       net.IP
	recorder     events.EventRecorder
	serviceMap   ServicesSnapshot
	endpointsMap EndpointsMap

	// Since converting probabilities (floats) to strings is expensive
	// and we are using only probabilities in the format of 1/n, we are
	// precomputing some number of those and cache for future reuse.
	precomputedProbabilities []string

	// The following buffers are used to reuse memory and avoid allocations
	// that are significantly impacting performance.
	iptablesData             *bytes.Buffer
	existingFilterChainsData *bytes.Buffer
	filterChains             util.LineBuffer
	filterRules              util.LineBuffer
	natChains                util.LineBuffer
	natRules                 util.LineBuffer

	// endpointChainsNumber is the total amount of endpointChains across all
	// services that we will generate (it is computed at the beginning of
	// syncProxyRules method). If that is large enough, comments in some
	// iptable rules are dropped to improve performance.
	endpointChainsNumber int

	// Values are as a parameter to select the interfaces where nodeport works.
	nodePortAddresses []string

	// Inject for test purpose.
	networkInterfacer NetworkInterfacer
	serviceChanges    *ServiceChangeTracker
	endpointsChanges  *EndpointChangeTracker
	localDetector     LocalTrafficDetector
	portsMap          map[utilnet.LocalPort]utilnet.Closeable
	iptInterface      util.Interface
}

var portMapper = &utilnet.ListenPortOpener

func NewIptables() *iptables {
	masqueradeBit := 14 //TODO: should it be fetched as flag etc?
	masqueradeValue := 1 << uint(masqueradeBit)

	return &iptables{
		serviceMap:               make(ServicesSnapshot),
		endpointsMap:             make(EndpointsMap),
		iptablesData:             bytes.NewBuffer(nil),
		existingFilterChainsData: bytes.NewBuffer(nil),
		filterChains:             util.LineBuffer{},
		filterRules:              util.LineBuffer{},
		natChains:                util.LineBuffer{},
		natRules:                 util.LineBuffer{},
		portsMap:                 make(map[utilnet.LocalPort]utilnet.Closeable),
		masqueradeAll:            masqueradeAll,
		masqueradeMark:           fmt.Sprintf("%#08x", masqueradeValue),
		localDetector:            NewNoOpLocalDetector(),
	}
}

func (t *iptables) sync() {
	defer wg.Done()
	// This is where the actual kube-proxy legacy logic takes over...

	// We assume that if this was called, we really want to sync them,
	// even if nothing changed in the meantime. In other words, callers are
	// responsible for detecting no-op changes and not calling this function.
	t.serviceMap.Update(t.serviceChanges)
	endpointUpdateResult := t.endpointsMap.Update(t.endpointsChanges)

	t.detectStaleConntrackEntries()
	klog.V(2).InfoS("Syncing iptables rules")

	// success := false
	// defer func() {
	// 	if !success {
	// 		klog.InfoS("Sync failed", "retryingTime", proxier.syncPeriod)
	// 		proxier.syncRunner.RetryAfter(proxier.syncPeriod)
	// 	}
	// }()

	t.ensureTopLevelChains()

	// previously we were doing initialization stuff
	// however at this point, were initialized, and this is the main logical
	// part of the proxy... This gets existing chains(not rules) for filter and nat.
	existingFilterChains := t.getExistingChains(util.TableFilter, t.existingFilterChainsData)
	existingNATChains := t.getExistingChains(util.TableNAT, t.iptablesData)

	// Reset all buffers used later.
	// This is to avoid memory reallocations and thus improve performance.
	t.resetAllChains()

	// Write iptables header lines to specific chain indicies...
	t.filterChains.Write("*filter")
	t.natChains.Write("*nat")

	// Make sure we keep stats for the top-level chains, if they existed
	// (which most should have because we created them above).
	t.createTopLevelChains(existingFilterChains, existingNATChains)

	// Install the kubernetes-specific postrouting rules. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	t.writePostRoutingMasqRules()

	// Accumulate NAT chains to keep.
	activeNATChains := map[util.Chain]bool{} // use a map as a set

	// Accumulate the set of local ports that we will be holding open once this update is complete
	replacementPortsMap := map[utilnet.LocalPort]utilnet.Closeable{}

	// // To avoid growing this slice, we arbitrarily set its size to 64,
	// // there is never more than that many arguments for a single line.
	// // Note that even if we go over 64, it will still be correct - it
	// // is just for efficiency, not correctness.
	args := make([]string, 64)

	t.endpointChainsNumber = 0
	for svcName := range t.serviceMap {
		if t.endpointsMap[svcName] == nil {
			continue
		}
		t.endpointChainsNumber += len(*(t.endpointsMap[svcName]))
	}

	localAddrSet := GetLocalAddrSet()
	nodeAddresses, err := GetNodeAddresses(t.nodePortAddresses, t.networkInterfacer)
	if err != nil {
		klog.ErrorS(err, "Failed to get node ip address matching nodeport cidrs, services with nodeport may not work as intended", "CIDRs", t.nodePortAddresses)
	}

	// Build rules for each service.
	for svcName, svcPortMap := range t.serviceMap {
		for _, svc := range svcPortMap {
			svcInfo, ok := svc.(*serviceInfo)
			if !ok {
				klog.ErrorS(nil, "Failed to cast serviceInfo", "svcName", svcName.String())
				continue
			}
			allEndpoints := t.endpointsMap[svcName]

			//TODO hope below one is not requires ,as per michael its handled in controller
			// Filtering for topology aware endpoints. This function will only
			// filter endpoints if appropriate feature gates are enabled and the
			// Service does not have conflicting configuration such as
			// externalTrafficPolicy=Local.
			// allEndpoints = FilterEndpoints(allEndpoints, svcInfo, proxier.nodeLabels)
			var hasEndpoints bool
			if allEndpoints != nil {
				hasEndpoints = len(*allEndpoints) > 0
			}
			endpoints, endpointChains := t.createServiceSpecificChains(svcInfo, activeNATChains, existingNATChains, allEndpoints)

			t.writeClusterIPRules(svcInfo, svcName, args[:0])
			t.writeExternalIPRules(svcInfo, svcName, args[:0], localAddrSet, replacementPortsMap)
			t.writeLoadBalancerRules(svcInfo, svcName, args[:0])
			t.writeNodePortsRules(svcInfo, nodeAddresses, svcName, localAddrSet, replacementPortsMap, args[:0])

			if !hasEndpoints {
				continue
			}

			readyEndpointChains, readyEndpoints, localReadyEndpointChains := t.getReadyEndpointsInfo(endpoints, endpointChains)
			t.writeEndpointRules(svcInfo, svcName, endpointChains, readyEndpointChains, endpoints, readyEndpoints, &args)

			// The logic below this applies only if this service is marked as OnlyLocal
			if svcInfo.NodeLocalExternal() {
				t.writeLocalExtTrafficPolicyRules(svcInfo, svcName, localReadyEndpointChains, args[:0])
			}
		}
	}
	// Delete chains no longer in use.
	t.deleteStaleChains(existingNATChains, activeNATChains)

	// Finally, tail-call to the nodeports chain.  This needs to be after all
	// other service portal rules.
	t.writeNodePortJumpRule(nodeAddresses, args[:0])
	t.writeMiscFilterRules()
	err = t.applyAllRules()
	if err != nil {
		klog.ErrorS(err, "Failed to execute iptables-restore")
		IptablesRestoreFailuresTotal.Inc()
		// Revert new local ports.
		klog.V(2).InfoS("Closing local ports after iptables-restore failure")
		RevertPorts(replacementPortsMap, t.portsMap)
		return
	}
	//	success = true

	for name, lastChangeTriggerTimes := range endpointUpdateResult.LastChangeTriggerTimes {
		for _, lastChangeTriggerTime := range lastChangeTriggerTimes {
			latency := SinceInSeconds(lastChangeTriggerTime)
			NetworkProgrammingLatency.Observe(latency)
			klog.V(4).InfoS("Network programming", "endpoint", klog.KRef(name.Namespace, name.Name), "elapsed", latency)
		}
	}

	// Close old local ports and save new ones.
	for k, v := range t.portsMap {
		if replacementPortsMap[k] == nil {
			v.Close()
		}
	}
	t.portsMap = replacementPortsMap
	t.cleanUp()
}

func (t *iptables) createServiceSpecificChains(svcInfo *serviceInfo, activeNATChains map[util.Chain]bool,
	existingNATChains map[util.Chain][]byte, allEndpoints *endpointsInfoByName) ([]*string, *[]util.Chain) {
	if allEndpoints != nil && len(*allEndpoints) > 0 {
		// Create the per-service chain, retaining counters if possible.
		t.copyExistingChains([]util.Chain{svcInfo.servicePortChainName}, existingNATChains, &t.natChains)
		activeNATChains[svcInfo.servicePortChainName] = true
	}

	if svcInfo.NodeLocalExternal() {
		// Only for services request OnlyLocal traffic
		// create the per-service LB chain, retaining counters if possible.
		t.copyExistingChains([]util.Chain{svcInfo.serviceLBChainName}, existingNATChains, &t.natChains)
		activeNATChains[svcInfo.serviceLBChainName] = true
	}

	// create service firewall chain
	if len(svcInfo.LoadBalancerIPStrings()) > 0 {
		t.copyExistingChains([]util.Chain{svcInfo.serviceFirewallChainName}, existingNATChains, &t.natChains)
		activeNATChains[svcInfo.serviceFirewallChainName] = true
	}
	return t.createEndpointsChain(svcInfo, allEndpoints, existingNATChains, activeNATChains)
}

func (t *iptables) createTopLevelChains(existingFilterChains map[util.Chain][]byte, existingNATChains map[util.Chain][]byte) {
	t.copyExistingChains([]util.Chain{kubeServicesChain, kubeExternalServicesChain, kubeForwardChain, kubeNodePortsChain},
		existingFilterChains, &t.filterChains)
	t.copyExistingChains([]util.Chain{kubeServicesChain, kubeNodePortsChain, kubePostroutingChain, KubeMarkMasqChain},
		existingNATChains, &t.natChains)
}

func (t *iptables) writePostRoutingMasqRules() {
	// Install the kubernetes-specific postrouting rules. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	// NB: THIS MUST MATCH the corresponding code in the kubelet
	t.natRules.Write(
		"-A", string(kubePostroutingChain),
		"-m", "mark", "!", "--mark", fmt.Sprintf("%s/%s", t.masqueradeMark, t.masqueradeMark),
		"-j", "RETURN",
	)
	// Clear the mark to avoid re-masquerading if the packet re-traverses the network stack.
	t.natRules.Write(
		"-A", string(kubePostroutingChain),
		// XOR proxier.masqueradeMark to unset it
		"-j", "MARK", "--xor-mark", t.masqueradeMark,
	)
	masqRule := []string{
		"-A", string(kubePostroutingChain),
		"-m", "comment", "--comment", `"kubernetes service traffic requiring SNAT"`,
		"-j", "MASQUERADE",
	}
	// TODO add logic for random-fully and iptables version logic eventually
	// assume we are on a newer iptables...
	// if HasRandomFully() {
	// 	masqRule = append(masqRule, "--random-fully")
	// }
	t.natRules.Write(masqRule)

	// Install the kubernetes-specific masquerade mark rule. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	t.natRules.Write(
		"-A", string(KubeMarkMasqChain),
		"-j", "MARK", "--or-mark", t.masqueradeMark,
	)
}

func (t *iptables) detectStaleConntrackEntries() {
	//TODO is this not required or should it move to kpng controller?
	// // We need to detect stale connections to UDP Services so we
	// // can clean dangling conntrack entries that can blackhole traffic.
	// conntrackCleanupServiceIPs := serviceUpdateResult.UDPStaleClusterIP
	// conntrackCleanupServiceNodePorts := sets.NewInt()
	// merge stale services gathered from updateEndpointsMap
	// an UDP service that changes from 0 to non-0 endpoints is considered stale.
	// for _, svcPortName := range endpointUpdateResult.StaleServiceNames {
	// 	if svcInfo, ok := t.serviceMap[svcPortName]; ok && svcInfo != nil && conntrack.IsClearConntrackNeeded(svcInfo.Protocol()) {
	// 		klog.V(2).InfoS("Stale service", "protocol", strings.ToLower(string(svcInfo.Protocol())), "svcPortName", svcPortName.String(), "clusterIP", svcInfo.ClusterIP().String())
	// 		conntrackCleanupServiceIPs.Insert(svcInfo.ClusterIP().String())
	// 		for _, extIP := range svcInfo.ExternalIPStrings() {
	// 			conntrackCleanupServiceIPs.Insert(extIP)
	// 		}
	// 		nodePort := svcInfo.NodePort()
	// 		if svcInfo.Protocol() == v1.ProtocolUDP && nodePort != 0 {
	// 			klog.V(2).Infof("Stale %s service NodePort %v -> %d", strings.ToLower(string(svcInfo.Protocol())), svcPortName, nodePort)
	// 			conntrackCleanupServiceNodePorts.Insert(nodePort)
	// 		}
	// 	}
	// }
}

func (t *iptables) deleteStaleChains(existingNATChains map[util.Chain][]byte, activeNATChains map[util.Chain]bool) {
	// Delete chains no longer in use.
	for chain := range existingNATChains {
		if !activeNATChains[chain] {
			chainString := string(chain)
			if !strings.HasPrefix(chainString, "KUBE-SVC-") && !strings.HasPrefix(chainString, "KUBE-SEP-") && !strings.HasPrefix(chainString, "KUBE-FW-") && !strings.HasPrefix(chainString, "KUBE-XLB-") {
				// Ignore chains that aren't ours.
				continue
			}
			// We must (as per iptables) write a chain-line for it, which has
			// the nice effect of flushing the chain.  Then we can remove the
			// chain.
			t.natChains.WriteBytes(existingNATChains[chain])
			t.natRules.Write("-X", chainString)
		}
	}

}

func (t *iptables) copyExistingChains(chains []util.Chain, existingChainData map[util.Chain][]byte, newChainData *util.LineBuffer) {
	// Make sure we keep stats for the top-level chains, if they existed
	// (which most should have because we created them above).
	for _, chainName := range chains {
		if chain, ok := existingChainData[chainName]; ok {
			newChainData.WriteBytes(chain)
		} else {
			newChainData.Write(util.MakeChainLine(chainName))
		}
	}
}

//writeClusterIPRules writes rules to reach svc chain from kube-services
func (t *iptables) writeClusterIPRules(svcInfo *serviceInfo, svcName types.NamespacedName, args []string) {
	svcChain := svcInfo.servicePortChainName
	protocol := strings.ToLower(svcInfo.Protocol().String())
	if val, ok := t.endpointsMap[svcName]; ok && len(*val) > 0 {
		args = append(args[:0],
			"-m", "comment", "--comment", fmt.Sprintf(`"%s cluster IP"`, svcInfo.serviceNameString),
			"-m", protocol, "-p", protocol,
			"-d", ToCIDR(svcInfo.ClusterIP()),
			"--dport", strconv.Itoa(svcInfo.Port()),
		)
		if t.masqueradeAll {
			t.natRules.Write("-A", string(svcChain), args, "-j", string(KubeMarkMasqChain))
		} else if t.localDetector.IsImplemented() { //TODO is this required?
			// This masquerades off-cluster traffic to a service VIP.  The idea
			// is that you can establish a static route for your Service range,
			// routing to any node, and that node will bridge into the Service
			// for you.  Since that might bounce off-node, we masquerade here.
			// If/when we support "Local" policy for VIPs, we should update this.
			t.natRules.Write("-A", string(svcChain), t.localDetector.JumpIfNotLocal(args, string(KubeMarkMasqChain)))
		}
		t.natRules.Write("-A", string(kubeServicesChain), args, "-j", string(svcChain))
	} else {
		// No endpoints.
		t.filterRules.Write(
			"-A", string(kubeServicesChain),
			"-m", "comment", "--comment", fmt.Sprintf(`"%s has no endpoints"`, svcInfo.serviceNameString),
			"-m", protocol, "-p", protocol,
			"-d", svcInfo.ClusterIP().String(),
			"--dport", strconv.Itoa(svcInfo.Port()),
			"-j", "REJECT",
		)
	}
}

//writeExternalIPRules writes rules in kube-services to jump to xlb/svc chain
func (t *iptables) writeExternalIPRules(svcInfo *serviceInfo, svcName types.NamespacedName, args []string,
	localAddrSet utilnet.IPSet, replacementPortsMap map[utilnet.LocalPort]utilnet.Closeable) {
	svcChain := svcInfo.servicePortChainName
	svcXlbChain := svcInfo.serviceLBChainName
	protocol := strings.ToLower(svcInfo.Protocol().String())
	for _, externalIP := range svcInfo.ExternalIPStrings() {
		// If the "external" IP happens to be an IP that is local to this
		// machine, hold the local port open so no other process can open it
		// (because the socket might open but it would never work).
		ipFamily := utilnet.IPv4
		if t.iptInterface.IsIPv6() {
			ipFamily = utilnet.IPv6
		}
		t.openPortLocally(protocol, localAddrSet, externalIP, svcInfo.Port(),
			ipFamily, "externalIP for "+svcInfo.serviceNameString, replacementPortsMap)

		if val, ok := t.endpointsMap[svcName]; ok && len(*val) > 0 {
			args = append(args[:0],
				"-m", "comment", "--comment", fmt.Sprintf(`"%s external IP"`, svcInfo.serviceNameString),
				"-m", protocol, "-p", protocol,
				"-d", ToCIDR(net.ParseIP(externalIP)),
				"--dport", strconv.Itoa(svcInfo.Port()),
			)

			destChain := svcXlbChain
			// We have to SNAT packets to external IPs if externalTrafficPolicy is cluster
			// and the traffic is NOT Local. Local traffic coming from Pods and Nodes will
			// be always forwarded to the corresponding Service, so no need to SNAT
			// If we can't differentiate the local traffic we always SNAT.
			if !svcInfo.NodeLocalExternal() {
				appendTo := []string{"-A", string(svcChain)}
				destChain = svcChain
				// This masquerades off-cluster traffic to a External IP.
				if t.localDetector.IsImplemented() {
					t.natRules.Write(appendTo, t.localDetector.JumpIfNotLocal(args, string(KubeMarkMasqChain)))
				} else {
					t.natRules.Write(appendTo, args, "-j", string(KubeMarkMasqChain))
				}
			}
			// Send traffic bound for external IPs to the service chain.
			t.natRules.Write("-A", string(kubeServicesChain), args, "-j", string(destChain))

		} else {
			// No endpoints.
			t.filterRules.Write(
				"-A", string(kubeExternalServicesChain),
				"-m", "comment", "--comment", fmt.Sprintf(`"%s has no endpoints"`, svcInfo.serviceNameString),
				"-m", protocol, "-p", protocol,
				"-d", net.ParseIP(externalIP),
				"--dport", strconv.Itoa(svcInfo.Port()),
				"-j", "REJECT",
			)
		}
	}
}

// writeLoadBalancerRules writes rules to FW chain to jump to svc/xlb and writes rule to jump to FW
// in kube-services
func (t *iptables) writeLoadBalancerRules(svcInfo *serviceInfo, svcName types.NamespacedName, args []string) {
	svcChain := svcInfo.servicePortChainName
	fwChain := svcInfo.serviceFirewallChainName
	svcXlbChain := svcInfo.serviceLBChainName
	protocol := strings.ToLower(svcInfo.Protocol().String())
	for _, ingress := range svcInfo.LoadBalancerIPStrings() {
		if ingress != "" {
			if val, ok := t.endpointsMap[svcName]; ok && len(*val) > 0 {

				// The service firewall rules are created based on ServiceSpec.loadBalancerSourceRanges field.
				// This currently works for loadbalancers that preserves source ips.
				// For loadbalancers which direct traffic to service NodePort, the firewall rules will not apply.

				args = append(args[:0],
					"-A", string(kubeServicesChain),
					"-m", "comment", "--comment", fmt.Sprintf(`"%s loadbalancer IP"`, svcInfo.serviceNameString),
					"-m", protocol, "-p", protocol,
					"-d", ToCIDR(net.ParseIP(ingress)),
					"--dport", strconv.Itoa(svcInfo.Port()),
				)
				// jump to service firewall chain
				t.natRules.Write(args, "-j", string(fwChain))

				args = append(args[:0],
					"-A", string(fwChain),
					"-m", "comment", "--comment", fmt.Sprintf(`"%s loadbalancer IP"`, svcInfo.serviceNameString),
				)

				// Each source match rule in the FW chain may jump to either the SVC or the XLB chain
				chosenChain := svcXlbChain
				// If we are proxying globally, we need to masquerade in case we cross nodes.
				// If we are proxying only locally, we can retain the source IP.
				if !svcInfo.NodeLocalExternal() {
					t.natRules.Write(args, "-j", string(KubeMarkMasqChain))
					chosenChain = svcChain
				}

				if len(svcInfo.LoadBalancerSourceRanges()) == 0 {
					// allow all sources, so jump directly to the KUBE-SVC or KUBE-XLB chain
					t.natRules.Write(args, "-j", string(chosenChain))
				} else {
					// firewall filter based on each source range
					allowFromNode := false
					for _, src := range svcInfo.LoadBalancerSourceRanges() {
						t.natRules.Write(args, "-s", src, "-j", string(chosenChain))
						_, cidr, err := net.ParseCIDR(src)
						if err != nil {
							klog.ErrorS(err, "Error parsing CIDR in LoadBalancerSourceRanges, dropping it", "cidr", cidr)
						} else if cidr.Contains(t.nodeIP) {
							allowFromNode = true
						}
					}
					// generally, ip route rule was added to intercept request to loadbalancer vip from the
					// loadbalancer's backend hosts. In this case, request will not hit the loadbalancer but loop back directly.
					// Need to add the following rule to allow request on host.
					if allowFromNode {
						t.natRules.Write(args, "-s", ingress, "-j", string(chosenChain))
					}
				}

				// If the packet was able to reach the end of firewall chain, then it did not get DNATed.
				// It means the packet cannot go thru the firewall, then mark it for DROP
				t.natRules.Write(args, "-j", string(KubeMarkDropChain))
			} else {
				// No endpoints.
				t.filterRules.Write(
					"-A", string(kubeExternalServicesChain),
					"-m", "comment", "--comment", fmt.Sprintf(`"%s has no endpoints"`, svcInfo.serviceNameString),
					"-m", protocol, "-p", protocol,
					"-d", ToCIDR(net.ParseIP(ingress)),
					"--dport", strconv.Itoa(svcInfo.Port()),
					"-j", "REJECT",
				)
			}
		}
	}
}

//writeNodePortsRules write rules to nodeports to jump to xlb/svc.
func (t *iptables) writeNodePortsRules(svcInfo *serviceInfo, nodeAddresses sets.String,
	svcName types.NamespacedName, localAddrSet utilnet.IPSet,
	replacementPortsMap map[utilnet.LocalPort]utilnet.Closeable, args []string) {
	//If we had more than 2 rules it might be
	// worthwhile to make a new per-service chain for nodeport rules, but
	// with just 2 rules it ends up being a waste and a cognitive burden.
	svcChain := svcInfo.servicePortChainName
	svcXlbChain := svcInfo.serviceLBChainName
	protocol := strings.ToLower(svcInfo.Protocol().String())
	ipFamily := utilnet.IPv4
	if t.iptInterface.IsIPv6() {
		ipFamily = utilnet.IPv6
	}

	if svcInfo.NodePort() != 0 && len(nodeAddresses) != 0 {
		// Hold the local port open so no other process can open it
		// (because the socket might open but it would never work).
		for address := range nodeAddresses {
			t.openPortLocally(protocol, localAddrSet, address, svcInfo.NodePort(),
				ipFamily, "nodePort for "+svcInfo.serviceNameString, replacementPortsMap)
		}

		if val, ok := t.endpointsMap[svcName]; ok && len(*val) > 0 {
			args = append(args[:0],
				"-m", "comment", "--comment", svcInfo.serviceNameString,
				"-m", protocol, "-p", protocol,
				"--dport", strconv.Itoa(svcInfo.NodePort()),
			)
			if !svcInfo.NodeLocalExternal() {
				// Nodeports need SNAT, unless they're local.
				t.natRules.Write("-A", string(svcChain), args, "-j", string(KubeMarkMasqChain))
				// Jump to the service chain.
				t.natRules.Write("-A", string(kubeNodePortsChain), args, "-j", string(svcChain))
			} else {
				// TODO: Make all nodePorts jump to the firewall chain.
				// Currently we only create it for loadbalancers (#33586).

				// Fix localhost martian source error
				loopback := "127.0.0.0/8"
				if t.iptInterface.IsIPv6() {
					loopback = "::1/128"
				}
				appendTo := []string{"-A", string(kubeNodePortsChain)}
				t.natRules.Write(appendTo, args, "-s", loopback, "-j", string(KubeMarkMasqChain))
				t.natRules.Write(appendTo, args, "-j", string(svcXlbChain))
			}
		} else {
			// No endpoints.
			t.filterRules.Write(
				"-A", string(kubeExternalServicesChain),
				"-m", "comment", "--comment", fmt.Sprintf(`"%s has no endpoints"`, svcInfo.serviceNameString),
				"-m", "addrtype", "--dst-type", "LOCAL",
				"-m", protocol, "-p", protocol,
				"--dport", strconv.Itoa(svcInfo.NodePort()),
				"-j", "REJECT",
			)
		}
	}
	// Capture healthCheckNodePorts.
	if svcInfo.HealthCheckNodePort() != 0 {
		// no matter if node has local endpoints, healthCheckNodePorts
		// need to add a rule to accept the incoming connection
		t.filterRules.Write(
			"-A", string(kubeNodePortsChain),
			"-m", "comment", "--comment", fmt.Sprintf(`"%s health check node port"`, svcInfo.serviceNameString),
			"-m", "tcp", "-p", "tcp",
			"--dport", strconv.Itoa(svcInfo.HealthCheckNodePort()),
			"-j", "ACCEPT",
		)
	}
}

//createEndpointsChain creates chains for each ep
func (t *iptables) createEndpointsChain(svcInfo *serviceInfo, allEndpoints *endpointsInfoByName,
	existingNATChains map[util.Chain][]byte, activeNATChains map[util.Chain]bool) ([]*string, *[]util.Chain) {
	endpoints := make([]*string, 0)
	endpointChains := make([]util.Chain, 0)
	protocol := strings.ToLower(svcInfo.Protocol().String())
	var endpointChain util.Chain
	if allEndpoints == nil {
		return nil, nil
	}
	for _, epInfo := range *allEndpoints {
		// epInfo, ok := ep.(*endpointsInfo)
		// if !ok {
		// 	klog.ErrorS(err, "Failed to cast endpointsInfo", "endpointsInfo", ep.String())
		// 	continue
		// }
		var ep string
		if t.iptInterface.IsIPv6() {
			if len(epInfo.IPs.V6) <= 0 {
				continue
			}
			ep = epInfo.IPs.V6[0]
		} else {
			if len(epInfo.IPs.V4) <= 0 {
				continue
			}
			ep = epInfo.IPs.V4[0]
		}
		endpoints = append(endpoints, &ep)

		endpointChain = servicePortEndpointChainName(svcInfo.serviceNameString, protocol, ep)
		endpointChains = append(endpointChains, endpointChain)

		// Create the endpoint chain, retaining counters if possible.
		t.copyExistingChains([]util.Chain{endpointChain}, existingNATChains, &t.natChains)
		activeNATChains[endpointChain] = true
	}
	return endpoints, &endpointChains
}

//writeEndpointRules writes rules to svc to jump to sep and rules to sep to dnat and loadbalance to actual ep ip
func (t *iptables) writeEndpointRules(svcInfo *serviceInfo, svcName types.NamespacedName, endpointChains *[]util.Chain,
	readyEndpointChains *[]util.Chain, endpoints []*string, readyEndpoints []*string, args *[]string) {
	// First write session affinity rules, if applicable.
	t.writeSessionAffinityRules(svcInfo, (*args)[:0], endpointChains, svcName)
	// Now write loadbalancing & DNAT rules.
	t.writeEndpointLBRules(svcInfo, svcName, readyEndpointChains, readyEndpoints, (*args)[:0])
	t.writeDNATRules(svcInfo, svcName, endpoints, endpointChains, (*args)[:0])
}

func (t *iptables) writeSessionAffinityRules(svcInfo *serviceInfo, args []string, endpointChains *[]util.Chain,
	svcName types.NamespacedName) {
	svcChain := svcInfo.servicePortChainName
	if svcInfo.SessionAffinity().ClientIP != nil {
		for _, endpointChain := range *endpointChains {
			args = append(args[:0],
				"-A", string(svcChain),
			)
			args = t.appendServiceCommentLocked(args, svcInfo.serviceNameString)
			args = append(args,
				"-m", "recent", "--name", string(endpointChain),
				"--rcheck", "--seconds", strconv.Itoa(int(svcInfo.SessionAffinity().ClientIP.ClientIP.TimeoutSeconds)), "--reap",
				"-j", string(endpointChain),
			)
			t.natRules.Write(args)
		}
	}
}

func (t *iptables) getReadyEndpointsInfo(endpoints []*string, endpointChains *[]util.Chain) (*[]util.Chain, []*string, *[]util.Chain) {
	//TODO: KPng EP doesnot have ready states.This logic needs to be checked.
	//I have removed the ready checks,else EP chains wont be added.
	// Firstly, categorize each endpoint into three buckets:
	//   1. all endpoints that are ready and NOT terminating.
	//   2. all endpoints that are local, ready and NOT terminating, and externalTrafficPolicy=Local
	//   3. all endpoints that are local, serving and terminating, and externalTrafficPolicy=Local
	readyEndpointChains := make([]util.Chain, 0)
	readyEndpointChains = readyEndpointChains[:0]
	readyEndpoints := make([]*string, 0)
	readyEndpoints = readyEndpoints[:0]
	localReadyEndpointChains := make([]util.Chain, 0)
	localReadyEndpointChains = localReadyEndpointChains[:0]
	//TODO: Check below line
	// localServingTerminatingEndpointChains := localServingTerminatingEndpointChains[:0]
	for i, endpointChain := range *endpointChains {
		// if endpoints[i].Ready {
		readyEndpointChains = append(readyEndpointChains, endpointChain)
		readyEndpoints = append(readyEndpoints, endpoints[i])
		// }

		// TODO: CHECK node local external how to check
		// if svc.NodeLocalExternal() && endpoints[i].IsLocal {
		// 	// if endpoints[i].Ready {
		// 	localReadyEndpointChains = append(localReadyEndpointChains, endpointChain)
		// 	// } else if endpoints[i].Serving && endpoints[i].Terminating {
		// 	// 	localServingTerminatingEndpointChains = append(localServingTerminatingEndpointChains, endpointChain)
		// 	// }
		// }
	}
	return &readyEndpointChains, readyEndpoints, &localReadyEndpointChains

}

func (t *iptables) writeEndpointLBRules(svcInfo *serviceInfo, svcName types.NamespacedName,
	readyEndpointChains *[]util.Chain, readyEndpoints []*string, args []string) {
	// Now write loadbalancing & DNAT rules.
	numReadyEndpoints := len(*readyEndpointChains)
	svcChain := svcInfo.servicePortChainName
	for i, endpointChain := range *readyEndpointChains {

		epIP := readyEndpoints[i]
		if *epIP == "" {
			// Error parsing this endpoint has been logged. Skip to next endpoint.
			continue
		}

		// Balancing rules in the per-service chain.
		args = append(args[:0], "-A", string(svcChain))
		args = t.appendServiceCommentLocked(args, svcInfo.serviceNameString)
		if i < (numReadyEndpoints - 1) {
			// Each rule is a probabilistic match.
			args = append(args,
				"-m", "statistic",
				"--mode", "random",
				"--probability", t.probability(numReadyEndpoints-i))
		}
		// The final (or only if n == 1) rule is a guaranteed match.
		args = append(args, "-j", string(endpointChain))
		t.natRules.Write(args)
	}
}

func (t *iptables) writeDNATRules(svcInfo *serviceInfo, svcName types.NamespacedName,
	endpoints []*string, endpointChains *[]util.Chain, args []string) {
	protocol := strings.ToLower(svcInfo.Protocol().String())
	for i, endpointChain := range *endpointChains {
		epIP := endpoints[i]
		if *epIP == "" {
			// Error parsing this endpoint has been logged. Skip to next endpoint.
			continue
		}
		// Rules in the per-endpoint chain.
		args = append(args[:0], "-A", string(endpointChain))
		args = t.appendServiceCommentLocked(args, svcInfo.serviceNameString)
		// Handle traffic that loops back to the originator with SNAT.
		t.natRules.Write(args,
			"-s", ToCIDR(net.ParseIP(*epIP)),
			"-j", string(KubeMarkMasqChain))
		// Update client-affinity lists.
		if svcInfo.SessionAffinity().ClientIP != nil {
			args = append(args, "-m", "recent", "--name", string(endpointChain), "--set")
		}
		// DNAT to final destination.
		args = append(args, "-m", protocol, "-p", protocol, "-j", "DNAT", "--to-destination", net.JoinHostPort(*endpoints[i], strconv.Itoa(svcInfo.TargetPort())))
		t.natRules.Write(args)
	}
}

func (t *iptables) writeLocalExtTrafficPolicyRules(svcInfo *serviceInfo, svcName types.NamespacedName, localReadyEndpointChains *[]util.Chain, args []string) {
	// First rule in the chain redirects all pod -> external VIP traffic to the
	// Service's ClusterIP instead. This happens whether or not we have local
	// endpoints; only if localDetector is implemented
	svcChain := svcInfo.servicePortChainName
	svcXlbChain := svcInfo.serviceLBChainName
	// First rule in the chain redirects all pod -> external VIP traffic to the
	// Service's ClusterIP instead. This happens whether or not we have local
	// endpoints; only if localDetector is implemented
	if t.localDetector.IsImplemented() {
		args = append(args[:0],
			"-A", string(svcXlbChain),
			"-m", "comment", "--comment",
			`"Redirect pods trying to reach external loadbalancer VIP to clusterIP"`,
		)
		t.natRules.Write(t.localDetector.JumpIfLocal(args, string(svcChain)))
	}

	// Next, redirect all src-type=LOCAL -> LB IP to the service chain for externalTrafficPolicy=Local
	// This allows traffic originating from the host to be redirected to the service correctly,
	// otherwise traffic to LB IPs are dropped if there are no local endpoints.
	args = append(args[:0], "-A", string(svcXlbChain))
	t.natRules.Write(args,
		"-m", "comment", "--comment", fmt.Sprintf(`"masquerade LOCAL traffic for %s LB IP"`, svcInfo.serviceNameString),
		"-m", "addrtype", "--src-type", "LOCAL", "-j", string(KubeMarkMasqChain))
	t.natRules.Write(args,
		"-m", "comment", "--comment", fmt.Sprintf(`"route LOCAL traffic for %s LB IP to service chain"`, svcInfo.serviceNameString),
		"-m", "addrtype", "--src-type", "LOCAL", "-j", string(svcChain))

	// Prefer local ready endpoint chains, but fall back to ready terminating if none exist
	localEndpointChains := localReadyEndpointChains
	// TODO: uncomment once 1.22 released
	// if utilfeature.DefaultFeatureGate.Enabled(features.ProxyTerminatingEndpoints) && len(localEndpointChains) == 0 {
	// 	localEndpointChains = localServingTerminatingEndpointChains
	// }

	numLocalEndpoints := len(*localEndpointChains)
	if numLocalEndpoints == 0 {
		// Blackhole all traffic since there are no local endpoints
		args = append(args[:0],
			"-A", string(svcXlbChain),
			"-m", "comment", "--comment",
			fmt.Sprintf(`"%s has no local endpoints"`, svcInfo.serviceNameString),
			"-j",
			string(KubeMarkDropChain),
		)
		t.natRules.Write(args)
	} else {
		// First write session affinity rules only over local endpoints, if applicable.
		if svcInfo.SessionAffinity().ClientIP != nil {
			for _, endpointChain := range *localEndpointChains {
				t.natRules.Write(
					"-A", string(svcXlbChain),
					"-m", "comment", "--comment", svcInfo.serviceNameString,
					"-m", "recent", "--name", string(endpointChain),
					"--rcheck", "--seconds", strconv.Itoa(int(svcInfo.SessionAffinity().ClientIP.ClientIP.TimeoutSeconds)), "--reap",
					"-j", string(endpointChain))
			}
		}

		// Setup probability filter rules only over local endpoints
		for i, endpointChain := range *localEndpointChains {
			// Balancing rules in the per-service chain.
			args = append(args[:0],
				"-A", string(svcXlbChain),
				"-m", "comment", "--comment",
				fmt.Sprintf(`"Balancing rule %d for %s"`, i, svcInfo.serviceNameString),
			)
			if i < (numLocalEndpoints - 1) {
				// Each rule is a probabilistic match.
				args = append(args,
					"-m", "statistic",
					"--mode", "random",
					"--probability", t.probability(numLocalEndpoints-i))
			}
			// The final (or only if n == 1) rule is a guaranteed match.
			args = append(args, "-j", string(endpointChain))
			t.natRules.Write(args)
		}
	}
}

//writeNodePortJumpRule writes rules to jump to NODEPORTS from kube-service for nodeips/zerocidr
func (t *iptables) writeNodePortJumpRule(nodeAddresses sets.String, args []string) {
	isIPv6 := t.iptInterface.IsIPv6()
	for address := range nodeAddresses {
		// TODO(thockin, m1093782566): If/when we have dual-stack support we will want to distinguish v4 from v6 zero-CIDRs.
		if IsZeroCIDR(address) {
			args = append(args[:0],
				"-A", string(kubeServicesChain),
				"-m", "comment", "--comment", `"kubernetes service nodeports; NOTE: this must be the last rule in this chain"`,
				"-m", "addrtype", "--dst-type", "LOCAL",
				"-j", string(kubeNodePortsChain))
			t.natRules.Write(args)
			// Nothing else matters after the zero CIDR.
			break
		}
		// Ignore IP addresses with incorrect version
		if isIPv6 && !utilnet.IsIPv6String(address) || !isIPv6 && utilnet.IsIPv6String(address) {
			klog.ErrorS(nil, "IP has incorrect IP version", "ip", address)
			continue
		}
		// create nodeport rules for each IP one by one
		args = append(args[:0],
			"-A", string(kubeServicesChain),
			"-m", "comment", "--comment", `"kubernetes service nodeports; NOTE: this must be the last rule in this chain"`,
			"-d", address,
			"-j", string(kubeNodePortsChain))
		t.natRules.Write(args)
	}
}

func (t *iptables) writeMiscFilterRules() {
	// Drop the packets in INVALID state, which would potentially cause
	// unexpected connection reset.
	// https://github.com/kubernetes/kubernetes/issues/74839
	t.filterRules.Write(
		"-A", string(kubeForwardChain),
		"-m", "conntrack",
		"--ctstate", "INVALID",
		"-j", "DROP",
	)

	// If the masqueradeMark has been added then we want to forward that same
	// traffic, this allows NodePort traffic to be forwarded even if the default
	// FORWARD policy is not accept.
	t.filterRules.Write(
		"-A", string(kubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding rules"`,
		"-m", "mark", "--mark", fmt.Sprintf("%s/%s", t.masqueradeMark, t.masqueradeMark),
		"-j", "ACCEPT",
	)

	// The following two rules ensure the traffic after the initial packet
	// accepted by the "kubernetes forwarding rules" rule above will be
	// accepted.
	t.filterRules.Write(
		"-A", string(kubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding conntrack pod source rule"`,
		"-m", "conntrack",
		"--ctstate", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	)
	t.filterRules.Write(
		"-A", string(kubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding conntrack pod destination rule"`,
		"-m", "conntrack",
		"--ctstate", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	)
}

func (t *iptables) openPortLocally(protocol string, localAddrSet utilnet.IPSet, ip string, port int, ipFamily utilnet.IPFamily, description string, replacementPortsMap map[utilnet.LocalPort]utilnet.Closeable) {
	if (v1.Protocol(protocol) != v1.ProtocolSCTP) && localAddrSet.Has(net.ParseIP(ip)) {
		lp := utilnet.LocalPort{
			Description: description,
			IP:          ip,
			IPFamily:    ipFamily,
			Port:        port,
			Protocol:    utilnet.Protocol(protocol),
		}
		if t.portsMap[lp] != nil {
			klog.V(4).InfoS("Port was open before and is still needed", "port", lp.String())
			replacementPortsMap[lp] = t.portsMap[lp]
		} else {
			socket, err := portMapper.OpenLocalPort(&lp)
			if err != nil {
				// msg := fmt.Sprintf("can't open port %s, skipping it", lp.String())

				// 	t.recorder.Eventf(
				// 		&v1.ObjectReference{
				// 			Kind:      "Node",
				// 			Name:      hostname, //TODO how to assign this
				// 			UID:       types.UID(hostname),
				// 			Namespace: "",
				// 		}, nil, v1.EventTypeWarning, err.Error(), "SyncProxyRules", msg)
				klog.ErrorS(err, "can't open port, skipping it", "port", lp.String())
			}
			klog.V(2).InfoS("Opened local port", "port", lp.String())
			replacementPortsMap[lp] = socket
		}
	}
}

func (t *iptables) applyAllRules() error {
	// Write the end-of-table markers.
	t.filterRules.Write("COMMIT")
	t.natRules.Write("COMMIT")
	// NOTE: NoFlushTables is used so we don't flush non-kubernetes chains in the table
	t.iptablesData.Reset()
	t.iptablesData.Write(t.filterChains.Bytes())
	t.iptablesData.Write(t.filterRules.Bytes())
	t.iptablesData.Write(t.natChains.Bytes())
	t.iptablesData.Write(t.natRules.Bytes())

	numberFilterIptablesRules := CountBytesLines(t.filterRules.Bytes())
	IptablesRulesTotal.WithLabelValues(string(util.TableFilter)).Set(float64(numberFilterIptablesRules))
	numberNatIptablesRules := CountBytesLines(t.natRules.Bytes())
	IptablesRulesTotal.WithLabelValues(string(util.TableNAT)).Set(float64(numberNatIptablesRules))

	klog.InfoS("Restoring iptables", "rules", string(t.iptablesData.Bytes()))
	err := t.iptInterface.RestoreAll(t.iptablesData.Bytes(), util.NoFlushTables, util.RestoreCounters)
	return err
}

func (t *iptables) resetAllChains() {
	t.filterChains.Reset()
	t.filterRules.Reset()
	t.natChains.Reset()
	t.natRules.Reset()
}

func (t *iptables) getExistingChains(tableType util.Table, buffer *bytes.Buffer) map[util.Chain][]byte {
	preexistingChains := make(map[util.Chain][]byte)
	buffer.Reset()
	err := t.iptInterface.SaveInto(tableType, buffer)
	if err != nil { // if we failed to get any rules
		klog.ErrorS(err, "Failed to execute iptables-save, syncing all rules")
	} else { // otherwise parse the output
		preexistingChains = util.GetChainLines(tableType, buffer.Bytes())
	}
	return preexistingChains
}

func (t *iptables) ensureTopLevelChains() {
	// Create and link the kube chains.  Note that "EnsureChain" will actually call iptables to make a chain if non-existent.
	for _, jump := range iptablesJumpChains {
		if _, err := t.iptInterface.EnsureChain(jump.table, jump.dstChain); err != nil {
			klog.ErrorS(err, "Failed to ensure chain exists", "table", jump.table, "chain", jump.dstChain)
			return
		}
		args := append(jump.extraArgs,
			"-m", "comment", "--comment", jump.comment,
			"-j", string(jump.dstChain),
		)
		if _, err := t.iptInterface.EnsureRule(util.Prepend, jump.table, jump.srcChain, args...); err != nil {
			klog.ErrorS(err, "Failed to ensure chain jumps", "table", jump.table, "srcChain", jump.srcChain, "dstChain", jump.dstChain)
			return
		}
	}

	// ensure KUBE-MARK-DROP chain exist but do not change any rules
	for _, ch := range iptablesEnsureChains {
		if _, err := t.iptInterface.EnsureChain(ch.table, ch.chain); err != nil {
			klog.ErrorS(err, "Failed to ensure chain exists", "table", ch.table, "chain", ch.chain)
			return
		}
	}
}

func (t *iptables) cleanUp() {
	//TODO: healthz below implementation required?
	// if healthzServer != nil {
	// 	healthzServer.Updated()
	// }
	// metrics.SyncProxyRulesLastTimestamp.SetToCurrentTime()

	// // Update service healthchecks.  The endpoints list might include services that are
	// // not "OnlyLocal", but the services list will not, and the serviceHealthServer
	// // will just drop those endpoints.
	// if err := proxier.serviceHealthServer.SyncServices(serviceUpdateResult.HCServiceNodePorts); err != nil {
	// 	klog.ErrorS(err, "Error syncing healthcheck services")
	// }
	// if err := proxier.serviceHealthServer.SyncEndpoints(endpointUpdateResult.HCEndpointsLocalIPSize); err != nil {
	// 	klog.ErrorS(err, "Error syncing healthcheck endpoints")
	// }

	// Finish housekeeping.
	// Clear stale conntrack entries for UDP Services, this has to be done AFTER the iptables rules are programmed.
	// TODO: these could be made more consistent.
	// TODO: conntrack cleanup commented for now as above also it wasnt included
	// klog.V(4).InfoS("Deleting conntrack stale entries for Services", "ips", conntrackCleanupServiceIPs.UnsortedList())
	// for _, svcIP := range conntrackCleanupServiceIPs.UnsortedList() {
	// 	if err := conntrack.ClearEntriesForIP(proxier.exec, svcIP, v1.ProtocolUDP); err != nil {
	// 		klog.ErrorS(err, "Failed to delete stale service connections", "ip", svcIP)
	// 	}
	// }
	// klog.V(4).InfoS("Deleting conntrack stale entries for Services", "nodeports", conntrackCleanupServiceNodePorts.UnsortedList())
	// for _, nodePort := range conntrackCleanupServiceNodePorts.UnsortedList() {
	// 	err := conntrack.ClearEntriesForPort(proxier.exec, nodePort, isIPv6, v1.ProtocolUDP)
	// 	if err != nil {
	// 		klog.ErrorS(err, "Failed to clear udp conntrack", "port", nodePort)
	// 	}
	// }
	// klog.V(4).InfoS("Deleting stale endpoint connections", "endpoints", endpointUpdateResult.StaleEndpoints)
	// deleteEndpointConnections(endpointUpdateResult.StaleEndpoints)
}

const endpointChainsNumberThreshold = 1000

// Assumes proxier.mu is held.
func (t *iptables) appendServiceCommentLocked(args []string, svcName string) []string {
	// Not printing these comments, can reduce size of iptables (in case of large
	// number of endpoints) even by 40%+. So if total number of endpoint chains
	// is large enough, we simply drop those comments.
	if t.endpointChainsNumber > endpointChainsNumberThreshold {
		return args
	}
	return append(args, "-m", "comment", "--comment", svcName)
}

// This assumes proxier.mu is held
func (t *iptables) probability(n int) string {
	if n >= len(t.precomputedProbabilities) {
		t.precomputeProbabilities(n)
	}
	return t.precomputedProbabilities[n]
}

// This assumes proxier.mu is held
func (t *iptables) precomputeProbabilities(numberOfPrecomputed int) {
	if len(t.precomputedProbabilities) == 0 {
		t.precomputedProbabilities = append(t.precomputedProbabilities, "<bad value>")
	}
	for i := len(t.precomputedProbabilities); i <= numberOfPrecomputed; i++ {
		t.precomputedProbabilities = append(t.precomputedProbabilities, t.computeProbability(i))
	}
}

func (t *iptables) computeProbability(n int) string {
	return fmt.Sprintf("%0.10f", 1.0/float64(n))
}

// After a UDP or SCTP endpoint has been removed, we must flush any pending conntrack entries to it, or else we
// risk sending more traffic to it, all of which will be lost.
// This assumes the proxier mutex is held
// TODO: move it to util
// func (proxier *Proxier) deleteEndpointConnections(connectionMap []proxy.ServiceEndpoint) {
// 	for _, epSvcPair := range connectionMap {
// 		if svcInfo, ok := proxier.serviceMap[epSvcPair.ServicePortName]; ok && conntrack.IsClearConntrackNeeded(svcInfo.Protocol()) {
// 			endpointIP := utilproxy.IPPart(epSvcPair.Endpoint)
// 			nodePort := svcInfo.NodePort()
// 			svcProto := svcInfo.Protocol()
// 			var err error
// 			if nodePort != 0 {
// 				err = conntrack.ClearEntriesForPortNAT(proxier.exec, endpointIP, nodePort, svcProto)
// 				if err != nil {
// 					klog.ErrorS(err, "Failed to delete nodeport-related endpoint connections", "servicePortName", epSvcPair.ServicePortName.String())
// 				}
// 			}
// 			err = conntrack.ClearEntriesForNAT(proxier.exec, svcInfo.ClusterIP().String(), endpointIP, svcProto)
// 			if err != nil {
// 				klog.ErrorS(err, "Failed to delete endpoint connections", "servicePortName", epSvcPair.ServicePortName.String())
// 			}
// 			for _, extIP := range svcInfo.ExternalIPStrings() {
// 				err := conntrack.ClearEntriesForNAT(proxier.exec, extIP, endpointIP, svcProto)
// 				if err != nil {
// 					klog.ErrorS(err, "Failed to delete endpoint connections for externalIP", "servicePortName", epSvcPair.ServicePortName.String(), "externalIP", extIP)
// 				}
// 			}
// 			for _, lbIP := range svcInfo.LoadBalancerIPStrings() {
// 				err := conntrack.ClearEntriesForNAT(proxier.exec, lbIP, endpointIP, svcProto)
// 				if err != nil {
// 					klog.ErrorS(err, "Failed to delete endpoint connections for LoadBalancerIP", "servicePortName", epSvcPair.ServicePortName.String(), "loadBalancerIP", lbIP)
// 				}
// 			}
// 		}
// 	}
// }
