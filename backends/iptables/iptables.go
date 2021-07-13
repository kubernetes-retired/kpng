package iptables

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/proxy/metrics"
	utilnode "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/utils/exec"
	utilnet "k8s.io/utils/net"
	"sigs.k8s.io/kpng/client"
)

var (
	flag = &pflag.FlagSet{}

	OnlyOutput = flag.Bool("only-output", false, "Only output the ipvsadm-restore file instead of calling ipvsadm-restore")
)

var mu sync.Mutex                // protects the following fields
var nodeLabels map[string]string //TODO: looks like can be removed as kpng controller shoujld do the work

// endpointsSynced, endpointSlicesSynced, and servicesSynced are set to true
// when corresponding objects are synced after startup. This is used to avoid
// updating iptables with some partial data after kube-proxy restart.
var endpointsSynced bool
var endpointSlicesSynced bool
var servicesSynced bool
var initialized int32
var syncPeriod time.Duration

// These are effectively const and do not need the mutex to be held.
var masqueradeAll bool
var masqueradeMark string
var hostname string
var nodeIP net.IP
var recorder events.EventRecorder
var serviceMap ServiceMap = make(ServiceMap)
var endpointsMap EndpointsMap = make(EndpointsMap)

// Since converting probabilities (floats) to strings is expensive
// and we are using only probabilities in the format of 1/n, we are
// precomputing some number of those and cache for future reuse.
var precomputedProbabilities []string

// The following buffers are used to reuse memory and avoid allocations
// that are significantly impacting performance.
var iptablesData *bytes.Buffer = bytes.NewBuffer(nil)
var existingFilterChainsData *bytes.Buffer = bytes.NewBuffer(nil)
var filterChains *bytes.Buffer = bytes.NewBuffer(nil)
var filterRules *bytes.Buffer = bytes.NewBuffer(nil)
var natChains *bytes.Buffer = bytes.NewBuffer(nil)
var natRules *bytes.Buffer = bytes.NewBuffer(nil)

// endpointChainsNumber is the total amount of endpointChains across all
// services that we will generate (it is computed at the beginning of
// syncProxyRules method). If that is large enough, comments in some
// iptable rules are dropped to improve performance.
var endpointChainsNumber int

// Values are as a parameter to select the interfaces where nodeport works.
var nodePortAddresses []string

// Inject for test purpose.
var networkInterfacer NetworkInterfacer
var serviceChanges *ServiceChangeTracker
var endpointsChanges *EndpointChangeTracker
var localDetector LocalTrafficDetector
var portsMap = make(map[utilnet.LocalPort]utilnet.Closeable)
var portMapper = &utilnet.ListenPortOpener

const (
	// the services chain
	kubeServicesChain Chain = "KUBE-SERVICES"

	// the external services chain
	kubeExternalServicesChain Chain = "KUBE-EXTERNAL-SERVICES"

	// the nodeports chain
	kubeNodePortsChain Chain = "KUBE-NODEPORTS"

	// the kubernetes postrouting chain
	kubePostroutingChain Chain = "KUBE-POSTROUTING"

	// KubeMarkMasqChain is the mark-for-masquerade chain
	KubeMarkMasqChain Chain = "KUBE-MARK-MASQ"

	// KubeMarkDropChain is the mark-for-drop chain
	KubeMarkDropChain Chain = "KUBE-MARK-DROP"

	// the kubernetes forward chain
	kubeForwardChain Chain = "KUBE-FORWARD"

	// kube proxy canary chain is used for monitoring rule reload
	kubeProxyCanaryChain Chain = "KUBE-PROXY-CANARY"
)

func PreRun() error {
	return nil
}

func BindFlags(flags *pflag.FlagSet) {
	flags.AddFlagSet(flag)
}

// internal struct for string service information
type serviceInfo struct {
	*BaseServiceInfo
	// The following fields are computed and stored for performance reasons.
	serviceNameString        string
	servicePortChainName     Chain
	serviceFirewallChainName Chain
	serviceLBChainName       Chain
}

// returns a new proxy.ServicePort which abstracts a serviceInfo
func newServiceInfo(port *v1.ServicePort, service *v1.Service, baseInfo *BaseServiceInfo) ServicePort {
	info := &serviceInfo{BaseServiceInfo: baseInfo}

	// Store the following for performance reasons.
	svcName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	svcPortName := ServicePortName{
		svcName,
		port.Name,
		info.protocol,
	}
	protocol := strings.ToLower(string(info.Protocol()))
	info.serviceNameString = svcPortName.String()
	info.servicePortChainName = servicePortChainName(info.serviceNameString, protocol)
	info.serviceFirewallChainName = serviceFirewallChainName(info.serviceNameString, protocol)
	info.serviceLBChainName = serviceLBChainName(info.serviceNameString, protocol)

	return info
}

// internal struct for endpoints information
type endpointsInfo struct {
	*BaseEndpointInfo
	// The following fields we lazily compute and store here for performance
	// reasons. If the protocol is the same as you expect it to be, then the
	// chainName can be reused, otherwise it should be recomputed.
	protocol  string
	chainName Chain
}

// returns a new proxy.Endpoint which abstracts a endpointsInfo
func newEndpointInfo(baseInfo *BaseEndpointInfo) Endpoint {
	return &endpointsInfo{BaseEndpointInfo: baseInfo}
}

// Equal overrides the Equal() function implemented by proxy.BaseEndpointInfo.
func (e *endpointsInfo) Equal(other Endpoint) bool {
	o, ok := other.(*endpointsInfo)
	if !ok {
		klog.ErrorS(nil, "Failed to cast endpointsInfo")
		return false
	}
	return e.Endpoint == o.Endpoint &&
		e.IsLocal == o.IsLocal &&
		e.protocol == o.protocol &&
		e.chainName == o.chainName
}

// Returns the endpoint chain name for a given endpointsInfo.
func (e *endpointsInfo) endpointChain(svcNameString, protocol string) Chain {
	if e.protocol != protocol {
		e.protocol = protocol
		e.chainName = servicePortEndpointChainName(svcNameString, protocol, e.Endpoint)
	}
	return e.chainName
}

func init() {
        masqueradeAll = true
	masqueradeBit := 14 //TODO: should it be fetched as flag etc?
	masqueradeValue := 1 << uint(masqueradeBit)
	masqueradeMark = fmt.Sprintf("%#08x", masqueradeValue)
	localDetector = NewNoOpLocalDetector()
	serviceChanges = NewServiceChangeTracker(newServiceInfo, v1.IPv4Protocol, recorder, nil)
	endpointsChanges = NewEndpointChangeTracker(hostname, newEndpointInfo, v1.IPv4Protocol, recorder, nil)
	//TODO : needs a better place. config.HostnameOverride is required here?
	var err error
	hostname, err = utilnode.GetHostname("")
	if err != nil {
		klog.Errorf("Could not get hostname: %s", err)
	}
}

// Callback receives the fullstate every time, so we can make the proxier.go functionality
// by rebuilding all the state as needed.  This is a port of the upstream kube proxy logic for iptables,
// which is very sophisticated.
func Callback(ch <-chan *client.ServiceEndpoints) {

	// 1) Copy the kpng datamodel into the iptables datamodel of serviceChanges and endpointsChanges.
	//TODO : The below code could be the only code in callBack.
	// The syncProxyRules
	//can be maintained in this file similar to existing kubeproxy.
	//Also callback could be made to receive only changes than full state by
	//providing an option in kpng to receive either full state or change(iptable would chose later).
	for serviceEndpoints := range ch {
		svc, err := ConvertToService(serviceEndpoints.Service)
		if err != nil {
			klog.Error(err)
			continue
		}
		serviceChanges.Update(nil, svc)
		v4Slice, v6Slice := ConvertToEPSlices(serviceEndpoints.Service, serviceEndpoints.Endpoints)
		if len(v4Slice.Endpoints) > 0 {
			endpointsChanges.EndpointSliceUpdate(v4Slice, false)
		}
		if len(v6Slice.Endpoints) > 0 {
			endpointsChanges.EndpointSliceUpdate(v6Slice, false)
		}
	}
	// end 1)

	// This is where the actual kube-proxy legacy logic takes over...

	// We assume that if this was called, we really want to sync them,
	// even if nothing changed in the meantime. In other words, callers are
	// responsible for detecting no-op changes and not calling this function.
	// serviceUpdateResult := serviceMap.Update(serviceChanges)
	serviceMap.Update(serviceChanges)
	endpointUpdateResult := endpointsMap.Update(endpointsChanges)

	//TODO is this not required? contrack cleanup
	// // We need to detect stale connections to UDP Services so we
	// // can clean dangling conntrack entries that can blackhole traffic.
	// conntrackCleanupServiceIPs := serviceUpdateResult.UDPStaleClusterIP
	// conntrackCleanupServiceNodePorts := sets.NewInt()
	// // merge stale services gathered from updateEndpointsMap
	// // an UDP service that changes from 0 to non-0 endpoints is considered stale.
	// for _, svcPortName := range endpointUpdateResult.StaleServiceNames {
	// 	if svcInfo, ok := proxier.serviceMap[svcPortName]; ok && svcInfo != nil && conntrack.IsClearConntrackNeeded(svcInfo.Protocol()) {
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
	// TODO, move these off for optimization somehow, and look
	// at making another backend for IPV6/Dual?
	execer := exec.New()
	iptInterface := New(execer, ProtocolIPv4)

	// Create and link the kube chains.  Note that "EnsureChain" will actually call iptables to make a chain if non-existent.
	for _, jump := range iptablesJumpChains {
		if _, err := iptInterface.EnsureChain(jump.table, jump.dstChain); err != nil {
			klog.ErrorS(err, "Failed to ensure chain exists", "table", jump.table, "chain", jump.dstChain)
			return
		}
		args := append(jump.extraArgs,
			"-m", "comment", "--comment", jump.comment,
			"-j", string(jump.dstChain),
		)
		if _, err := iptInterface.EnsureRule(Prepend, jump.table, jump.srcChain, args...); err != nil {
			klog.ErrorS(err, "Failed to ensure chain jumps", "table", jump.table, "srcChain", jump.srcChain, "dstChain", jump.dstChain)
			return
		}
	}

	// ensure KUBE-MARK-DROP chain exist but do not change any rules
	for _, ch := range iptablesEnsureChains {
		if _, err := iptInterface.EnsureChain(ch.table, ch.chain); err != nil {
			klog.ErrorS(err, "Failed to ensure chain exists", "table", ch.table, "chain", ch.chain)
			return
		}
	}

	// previously we were doing initialization stuff
	// however at this point, were initialized, and this is the main logical
	// part of the proxy...
	preexistingFilterChains := make(map[Chain][]byte)
	existingFilterChainsData.Reset()
	err := iptInterface.SaveInto(TableFilter, existingFilterChainsData)
	if err != nil { // if we failed to get any rules
		klog.ErrorS(err, "Failed to execute iptables-save, syncing all rules")
	} else { // otherwise parse the output
		preexistingFilterChains = GetChainLines(TableFilter, existingFilterChainsData.Bytes())
	}

	existingNATChains := make(map[Chain][]byte)
	iptablesData.Reset()
	err = iptInterface.SaveInto(TableNAT, iptablesData)
	if err != nil { // if we failed to get any rules
		klog.ErrorS(err, "Failed to execute iptables-save, syncing all rules")
	} else { // otherwise parse the output
		existingNATChains = GetChainLines(TableNAT, iptablesData.Bytes())
	}

	// Reset all buffers used later.
	// This is to avoid memory reallocations and thus improve performance.
	filterChains.Reset()
	filterRules.Reset()
	natChains.Reset()
	natRules.Reset()

	// Write iptables header lines to specific chain indicies...
	WriteLine(filterChains, "*filter")
	WriteLine(natChains, "*nat")

	// Make sure we keep stats for the top-level chains, if they existed
	// (which most should have because we created them above).
	for _, chainName := range []Chain{kubeServicesChain, kubeExternalServicesChain, kubeForwardChain, kubeNodePortsChain} {
		if chain, ok := preexistingFilterChains[chainName]; ok {
			WriteBytesLine(filterChains, chain)
		} else {
			WriteLine(filterChains, MakeChainLine(chainName))
		}
	}
	for _, chainName := range []Chain{kubeServicesChain, kubeNodePortsChain, kubePostroutingChain, KubeMarkMasqChain} {
		if chain, ok := existingNATChains[chainName]; ok {
			WriteBytesLine(natChains, chain)
		} else {
			WriteLine(natChains, MakeChainLine(chainName))
		}
	}

	// Install the kubernetes-specific postrouting rules. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	// NB: THIS MUST MATCH the corresponding code in the kubelet
	WriteLine(natRules, []string{
		"-A", string(kubePostroutingChain),
		"-m", "mark", "!", "--mark", fmt.Sprintf("%s/%s", masqueradeMark, masqueradeMark),
		"-j", "RETURN",
	}...)
	// Clear the mark to avoid re-masquerading if the packet re-traverses the network stack.
	WriteLine(natRules, []string{
		"-A", string(kubePostroutingChain),
		// XOR proxier.masqueradeMark to unset it
		"-j", "MARK", "--xor-mark", masqueradeMark,
	}...)
	masqRule := []string{
		"-A", string(kubePostroutingChain),
		"-m", "comment", "--comment", `"kubernetes service traffic requiring SNAT"`,
		"-j", "MASQUERADE",
	}
	// TODO add logic for random-fully and iptables version logic eventually
	// assume we are on a newer iptables...
	//if iptables.HasRandomFully() {
	//	masqRule = append(masqRule, "--random-fully")
	//}
	WriteLine(natRules, masqRule...)

	// Install the kubernetes-specific masquerade mark rule. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	WriteLine(natRules, []string{
		"-A", string(KubeMarkMasqChain),
		"-j", "MARK", "--or-mark", masqueradeMark,
	}...)

	// Accumulate NAT chains to keep.
	activeNATChains := map[Chain]bool{} // use a map as a set

	// Accumulate the set of local ports that we will be holding open once this update is complete
	replacementPortsMap := map[utilnet.LocalPort]utilnet.Closeable{}

	// We are creating those slices ones here to avoid memory reallocations
	// in every loop. Note that reuse the memory, instead of doing:
	//   slice = <some new slice>
	// you should always do one of the below:
	//   slice = slice[:0] // and then append to it
	//   slice = append(slice[:0], ...)
	endpoints := make([]*endpointsInfo, 0)
	endpointChains := make([]Chain, 0)
	readyEndpoints := make([]*endpointsInfo, 0)
	readyEndpointChains := make([]Chain, 0)
	localReadyEndpointChains := make([]Chain, 0)
	// localServingTerminatingEndpointChains := make([]Chain, 0)
	// To avoid growing this slice, we arbitrarily set its size to 64,
	// there is never more than that many arguments for a single line.
	// Note that even if we go over 64, it will still be correct - it
	// is just for efficiency, not correctness.
	args := make([]string, 64)

	endpointChainsNumber = 0
	for svcName := range serviceMap {
		endpointChainsNumber += len(endpointsMap[svcName])
	}

	localAddrSet := GetLocalAddrSet()

	nodeAddresses, err := GetNodeAddresses(nodePortAddresses, networkInterfacer)
	if err != nil {
		klog.ErrorS(err, "Failed to get node ip address matching nodeport cidrs, services with nodeport may not work as intended", "CIDRs", nodePortAddresses)
	}

	// Build rules for each service.
	for svcName, svc := range serviceMap {
		svcInfo, ok := svc.(*serviceInfo)
		if !ok {
			klog.ErrorS(nil, "Failed to cast serviceInfo", "svcName", svcName.String())
			continue
		}
		isIPv6 := utilnet.IsIPv6(svcInfo.ClusterIP())
		localPortIPFamily := utilnet.IPv4
		if isIPv6 {
			localPortIPFamily = utilnet.IPv6
		}
		protocol := strings.ToLower(string(svcInfo.Protocol()))
		svcNameString := svcInfo.serviceNameString
		klog.Info("CURRENT SVC:", svcNameString)
		allEndpoints := endpointsMap[svcName]
		klog.Info("EPS:", allEndpoints)

		//TODO hope below one is not requires ,as per michael its handled in controller
		// Filtering for topology aware endpoints. This function will only
		// filter endpoints if appropriate feature gates are enabled and the
		// Service does not have conflicting configuration such as
		// externalTrafficPolicy=Local.
		// allEndpoints = FilterEndpoints(allEndpoints, svcInfo, proxier.nodeLabels)
		hasEndpoints := len(allEndpoints) > 0

		svcChain := svcInfo.servicePortChainName
		if hasEndpoints {
			// Create the per-service chain, retaining counters if possible.
			if chain, ok := existingNATChains[svcChain]; ok {
				WriteBytesLine(natChains, chain)
			} else {
				WriteLine(natChains, MakeChainLine(svcChain))
			}
			activeNATChains[svcChain] = true
		}

		svcXlbChain := svcInfo.serviceLBChainName
		if svcInfo.NodeLocalExternal() {
			// Only for services request OnlyLocal traffic
			// create the per-service LB chain, retaining counters if possible.
			if lbChain, ok := existingNATChains[svcXlbChain]; ok {
				WriteBytesLine(natChains, lbChain)
			} else {
				WriteLine(natChains, MakeChainLine(svcXlbChain))
			}
			activeNATChains[svcXlbChain] = true
		}

		// Capture the clusterIP.
		if hasEndpoints {

			args = append(args[:0],
				"-m", "comment", "--comment", fmt.Sprintf(`"%s cluster IP"`, svcNameString),
				"-m", protocol, "-p", protocol,
				"-d", ToCIDR(svcInfo.ClusterIP()),
				"--dport", strconv.Itoa(svcInfo.Port()),
			)
			klog.Info("WRITING RULES FOR CLUSTERIP:", args)
			if masqueradeAll {
				WriteRuleLine(natRules, string(svcChain), append(args, "-j", string(KubeMarkMasqChain))...)
			} else if localDetector.IsImplemented() { //TODO is this required?
				// This masquerades off-cluster traffic to a service VIP.  The idea
				// is that you can establish a static route for your Service range,
				// routing to any node, and that node will bridge into the Service
				// for you.  Since that might bounce off-node, we masquerade here.
				// If/when we support "Local" policy for VIPs, we should update this.
				WriteRuleLine(natRules, string(svcChain), localDetector.JumpIfNotLocal(args, string(KubeMarkMasqChain))...)
			}
			WriteRuleLine(natRules, string(kubeServicesChain), append(args, "-j", string(svcChain))...)
		} else {
			// No endpoints.
			WriteLine(filterRules,
				"-A", string(kubeServicesChain),
				"-m", "comment", "--comment", fmt.Sprintf(`"%s has no endpoints"`, svcNameString),
				"-m", protocol, "-p", protocol,
				"-d", ToCIDR(svcInfo.ClusterIP()),
				"--dport", strconv.Itoa(svcInfo.Port()),
				"-j", "REJECT",
			)
		}

		// Capture externalIPs.
		for _, externalIP := range svcInfo.ExternalIPStrings() {
			// If the "external" IP happens to be an IP that is local to this
			// machine, hold the local port open so no other process can open it
			// (because the socket might open but it would never work).
			if (svcInfo.Protocol() != v1.ProtocolSCTP) && localAddrSet.Has(net.ParseIP(externalIP)) {
				lp := utilnet.LocalPort{
					Description: "externalIP for " + svcNameString,
					IP:          externalIP,
					IPFamily:    localPortIPFamily,
					Port:        svcInfo.Port(),
					Protocol:    utilnet.Protocol(svcInfo.Protocol()),
				}
				if portsMap[lp] != nil {
					klog.V(4).InfoS("Port was open before and is still needed", "port", lp.String())
					replacementPortsMap[lp] = portsMap[lp]
				} else {
					socket, err := portMapper.OpenLocalPort(&lp)
					if err != nil {
						msg := fmt.Sprintf("can't open port %s, skipping it", lp.String())

						recorder.Eventf(
							&v1.ObjectReference{
								Kind:      "Node",
								Name:      hostname, //TODO how to assign this
								UID:       types.UID(hostname),
								Namespace: "",
							}, nil, v1.EventTypeWarning, err.Error(), "SyncProxyRules", msg)
						klog.ErrorS(err, "can't open port, skipping it", "port", lp.String())
						continue
					}
					klog.V(2).InfoS("Opened local port", "port", lp.String())
					replacementPortsMap[lp] = socket
				}
			}

			if hasEndpoints {
				args = append(args[:0],
					"-m", "comment", "--comment", fmt.Sprintf(`"%s external IP"`, svcNameString),
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
					destChain = svcChain
					// This masquerades off-cluster traffic to a External IP.
					if localDetector.IsImplemented() {
						WriteRuleLine(natRules, string(svcChain), localDetector.JumpIfNotLocal(args, string(KubeMarkMasqChain))...)
					} else {
						WriteRuleLine(natRules, string(svcChain), append(args, "-j", string(KubeMarkMasqChain))...)
					}
				}
				// Send traffic bound for external IPs to the service chain.
				WriteRuleLine(natRules, string(kubeServicesChain), append(args, "-j", string(destChain))...)

			} else {
				// No endpoints.
				WriteLine(filterRules,
					"-A", string(kubeExternalServicesChain),
					"-m", "comment", "--comment", fmt.Sprintf(`"%s has no endpoints"`, svcNameString),
					"-m", protocol, "-p", protocol,
					"-d", ToCIDR(net.ParseIP(externalIP)),
					"--dport", strconv.Itoa(svcInfo.Port()),
					"-j", "REJECT",
				)
			}
		}

		//TODO:   loadBalancerSourceRanges is not taken in kpng service
		// Capture load-balancer ingress.
		fwChain := svcInfo.serviceFirewallChainName
		for _, ingress := range svcInfo.LoadBalancerIPStrings() {
			if ingress != "" {
				if hasEndpoints {
					// create service firewall chain
					if chain, ok := existingNATChains[fwChain]; ok {
						WriteBytesLine(natChains, chain)
					} else {
						WriteLine(natChains, MakeChainLine(fwChain))
					}
					activeNATChains[fwChain] = true
					// The service firewall rules are created based on ServiceSpec.loadBalancerSourceRanges field.
					// This currently works for loadbalancers that preserves source ips.
					// For loadbalancers which direct traffic to service NodePort, the firewall rules will not apply.

					args = append(args[:0],
						"-A", string(kubeServicesChain),
						"-m", "comment", "--comment", fmt.Sprintf(`"%s loadbalancer IP"`, svcNameString),
						"-m", protocol, "-p", protocol,
						"-d", ToCIDR(net.ParseIP(ingress)),
						"--dport", strconv.Itoa(svcInfo.Port()),
					)
					// jump to service firewall chain
					WriteLine(natRules, append(args, "-j", string(fwChain))...)

					args = append(args[:0],
						"-A", string(fwChain),
						"-m", "comment", "--comment", fmt.Sprintf(`"%s loadbalancer IP"`, svcNameString),
					)

					// Each source match rule in the FW chain may jump to either the SVC or the XLB chain
					chosenChain := svcXlbChain
					// If we are proxying globally, we need to masquerade in case we cross nodes.
					// If we are proxying only locally, we can retain the source IP.
					if !svcInfo.NodeLocalExternal() {
						WriteLine(natRules, append(args, "-j", string(KubeMarkMasqChain))...)
						chosenChain = svcChain
					}

					if len(svcInfo.LoadBalancerSourceRanges()) == 0 {
						// allow all sources, so jump directly to the KUBE-SVC or KUBE-XLB chain
						WriteLine(natRules, append(args, "-j", string(chosenChain))...)
					} else {
						// firewall filter based on each source range
						allowFromNode := false
						for _, src := range svcInfo.LoadBalancerSourceRanges() {
							WriteLine(natRules, append(args, "-s", src, "-j", string(chosenChain))...)
							_, cidr, err := net.ParseCIDR(src)
							if err != nil {
								klog.ErrorS(err, "Error parsing CIDR in LoadBalancerSourceRanges, dropping it", "cidr", cidr)
							} else if cidr.Contains(nodeIP) {
								allowFromNode = true
							}
						}
						// generally, ip route rule was added to intercept request to loadbalancer vip from the
						// loadbalancer's backend hosts. In this case, request will not hit the loadbalancer but loop back directly.
						// Need to add the following rule to allow request on host.
						if allowFromNode {
							WriteLine(natRules, append(args, "-s", ToCIDR(net.ParseIP(ingress)), "-j", string(chosenChain))...)
						}
					}

					// If the packet was able to reach the end of firewall chain, then it did not get DNATed.
					// It means the packet cannot go thru the firewall, then mark it for DROP
					WriteLine(natRules, append(args, "-j", string(KubeMarkDropChain))...)
				} else {
					// No endpoints.
					WriteLine(filterRules,
						"-A", string(kubeExternalServicesChain),
						"-m", "comment", "--comment", fmt.Sprintf(`"%s has no endpoints"`, svcNameString),
						"-m", protocol, "-p", protocol,
						"-d", ToCIDR(net.ParseIP(ingress)),
						"--dport", strconv.Itoa(svcInfo.Port()),
						"-j", "REJECT",
					)
				}
			}
		}

		// Capture nodeports.  If we had more than 2 rules it might be
		// worthwhile to make a new per-service chain for nodeport rules, but
		// with just 2 rules it ends up being a waste and a cognitive burden.
		if svcInfo.NodePort() != 0 {
			// Hold the local port open so no other process can open it
			// (because the socket might open but it would never work).
			if len(nodeAddresses) == 0 {
				continue
			}

			lps := make([]utilnet.LocalPort, 0)
			for address := range nodeAddresses {
				lp := utilnet.LocalPort{
					Description: "nodePort for " + svcNameString,
					IP:          address,
					IPFamily:    localPortIPFamily,
					Port:        svcInfo.NodePort(),
					Protocol:    utilnet.Protocol(svcInfo.Protocol()),
				}
				if IsZeroCIDR(address) {
					// Empty IP address means all
					lp.IP = ""
					lps = append(lps, lp)
					// If we encounter a zero CIDR, then there is no point in processing the rest of the addresses.
					break
				}
				lps = append(lps, lp)
			}

			// For ports on node IPs, open the actual port and hold it.
			for _, lp := range lps {
				if portsMap[lp] != nil {
					klog.V(4).InfoS("Port was open before and is still needed", "port", lp.String())
					replacementPortsMap[lp] = portsMap[lp]
				} else if svcInfo.Protocol() != v1.ProtocolSCTP {
					socket, err := portMapper.OpenLocalPort(&lp)
					if err != nil {
						msg := fmt.Sprintf("can't open port %s, skipping it", lp.String())

						recorder.Eventf(
							&v1.ObjectReference{
								Kind:      "Node",
								Name:      hostname,
								UID:       types.UID(hostname),
								Namespace: "",
							}, nil, v1.EventTypeWarning, err.Error(), "SyncProxyRules", msg)
						klog.ErrorS(err, "can't open port, skipping it", "port", lp.String())
						continue
					}
					klog.V(2).InfoS("Opened local port", "port", lp.String())
					replacementPortsMap[lp] = socket
				}
			}

			if hasEndpoints {
				args = append(args[:0],
					"-m", "comment", "--comment", svcNameString,
					"-m", protocol, "-p", protocol,
					"--dport", strconv.Itoa(svcInfo.NodePort()),
				)
				if !svcInfo.NodeLocalExternal() {
					// Nodeports need SNAT, unless they're local.
					WriteRuleLine(natRules, string(svcChain), append(args, "-j", string(KubeMarkMasqChain))...)
					// Jump to the service chain.
					WriteRuleLine(natRules, string(kubeNodePortsChain), append(args, "-j", string(svcChain))...)
				} else {
					// TODO: Make all nodePorts jump to the firewall chain.
					// Currently we only create it for loadbalancers (#33586).

					// Fix localhost martian source error
					loopback := "127.0.0.0/8"
					if isIPv6 {
						loopback = "::1/128"
					}
					WriteRuleLine(natRules, string(kubeNodePortsChain), append(args, "-s", loopback, "-j", string(KubeMarkMasqChain))...)
					WriteRuleLine(natRules, string(kubeNodePortsChain), append(args, "-j", string(svcXlbChain))...)
				}
			} else {
				// No endpoints.
				WriteLine(filterRules,
					"-A", string(kubeExternalServicesChain),
					"-m", "comment", "--comment", fmt.Sprintf(`"%s has no endpoints"`, svcNameString),
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
			WriteLine(filterRules,
				"-A", string(kubeNodePortsChain),
				"-m", "comment", "--comment", fmt.Sprintf(`"%s health check node port"`, svcNameString),
				"-m", "tcp", "-p", "tcp",
				"--dport", strconv.Itoa(svcInfo.HealthCheckNodePort()),
				"-j", "ACCEPT",
			)
		}

		if !hasEndpoints {
			continue
		}

		// Generate the per-endpoint chains.  We do this in multiple passes so we
		// can group rules together.
		// These two slices parallel each other - keep in sync
		endpoints = endpoints[:0]
		endpointChains = endpointChains[:0]
		var endpointChain Chain
		for _, ep := range allEndpoints {
			epInfo, ok := ep.(*endpointsInfo)
			if !ok {
				klog.ErrorS(err, "Failed to cast endpointsInfo", "endpointsInfo", ep.String())
				continue
			}

			endpoints = append(endpoints, epInfo)
			endpointChain = epInfo.endpointChain(svcNameString, protocol)
			endpointChains = append(endpointChains, endpointChain)

			// Create the endpoint chain, retaining counters if possible.
			if chain, ok := existingNATChains[endpointChain]; ok {
				WriteBytesLine(natChains, chain)
			} else {
				WriteLine(natChains, MakeChainLine(endpointChain))
			}
			activeNATChains[endpointChain] = true
		}

		// First write session affinity rules, if applicable.
		if svcInfo.SessionAffinityType() == v1.ServiceAffinityClientIP {
			for _, endpointChain := range endpointChains {
				args = append(args[:0],
					"-A", string(svcChain),
				)
				args = appendServiceCommentLocked(args, svcNameString)
				args = append(args,
					"-m", "recent", "--name", string(endpointChain),
					"--rcheck", "--seconds", strconv.Itoa(svcInfo.StickyMaxAgeSeconds()), "--reap",
					"-j", string(endpointChain),
				)
				WriteLine(natRules, args...)
			}
		}

		//TODO: KPng EP doesnot have ready states.This logic needs to be checked.
		//I have removed the ready checks,else EP chains wont be added.
		// Firstly, categorize each endpoint into three buckets:
		//   1. all endpoints that are ready and NOT terminating.
		//   2. all endpoints that are local, ready and NOT terminating, and externalTrafficPolicy=Local
		//   3. all endpoints that are local, serving and terminating, and externalTrafficPolicy=Local
		readyEndpointChains = readyEndpointChains[:0]
		readyEndpoints := readyEndpoints[:0]
		localReadyEndpointChains := localReadyEndpointChains[:0]
		// localServingTerminatingEndpointChains := localServingTerminatingEndpointChains[:0]
		for i, endpointChain := range endpointChains {
			// if endpoints[i].Ready {
			readyEndpointChains = append(readyEndpointChains, endpointChain)
			readyEndpoints = append(readyEndpoints, endpoints[i])
			// }

			if svc.NodeLocalExternal() && endpoints[i].IsLocal {
				// if endpoints[i].Ready {
				localReadyEndpointChains = append(localReadyEndpointChains, endpointChain)
				// } else if endpoints[i].Serving && endpoints[i].Terminating {
				// 	localServingTerminatingEndpointChains = append(localServingTerminatingEndpointChains, endpointChain)
				// }
			}
		}

		// Now write loadbalancing & DNAT rules.
		numReadyEndpoints := len(readyEndpointChains)
		for i, endpointChain := range readyEndpointChains {
			epIP := readyEndpoints[i].IP()
			if epIP == "" {
				// Error parsing this endpoint has been logged. Skip to next endpoint.
				continue
			}

			// Balancing rules in the per-service chain.
			args = append(args[:0], "-A", string(svcChain))
			args = appendServiceCommentLocked(args, svcNameString)
			if i < (numReadyEndpoints - 1) {
				// Each rule is a probabilistic match.
				args = append(args,
					"-m", "statistic",
					"--mode", "random",
					"--probability", probability(numReadyEndpoints-i))
			}
			// The final (or only if n == 1) rule is a guaranteed match.
			args = append(args, "-j", string(endpointChain))
			WriteLine(natRules, args...)
		}

		// Every endpoint gets a chain, regardless of its state. This is required later since we may
		// want to jump to endpoint chains that are terminating.
		for i, endpointChain := range endpointChains {
			epIP := endpoints[i].IP()
			if epIP == "" {
				// Error parsing this endpoint has been logged. Skip to next endpoint.
				continue
			}

			// Rules in the per-endpoint chain.
			args = append(args[:0], "-A", string(endpointChain))
			args = appendServiceCommentLocked(args, svcNameString)
			// Handle traffic that loops back to the originator with SNAT.
			WriteLine(natRules, append(args,
				"-s", ToCIDR(net.ParseIP(epIP)),
				"-j", string(KubeMarkMasqChain))...)
			// Update client-affinity lists.
			if svcInfo.SessionAffinityType() == v1.ServiceAffinityClientIP {
				args = append(args, "-m", "recent", "--name", string(endpointChain), "--set")
			}
			// DNAT to final destination.
			args = append(args, "-m", protocol, "-p", protocol, "-j", "DNAT", "--to-destination", endpoints[i].Endpoint)
			WriteLine(natRules, args...)
		}

		// The logic below this applies only if this service is marked as OnlyLocal
		if !svcInfo.NodeLocalExternal() {
			continue
		}

		// First rule in the chain redirects all pod -> external VIP traffic to the
		// Service's ClusterIP instead. This happens whether or not we have local
		// endpoints; only if localDetector is implemented
		if localDetector.IsImplemented() {
			args = append(args[:0],
				"-A", string(svcXlbChain),
				"-m", "comment", "--comment",
				`"Redirect pods trying to reach external loadbalancer VIP to clusterIP"`,
			)
			WriteLine(natRules, localDetector.JumpIfLocal(args, string(svcChain))...)
		}

		// Next, redirect all src-type=LOCAL -> LB IP to the service chain for externalTrafficPolicy=Local
		// This allows traffic originating from the host to be redirected to the service correctly,
		// otherwise traffic to LB IPs are dropped if there are no local endpoints.
		args = append(args[:0], "-A", string(svcXlbChain))
		WriteLine(natRules, append(args,
			"-m", "comment", "--comment", fmt.Sprintf(`"masquerade LOCAL traffic for %s LB IP"`, svcNameString),
			"-m", "addrtype", "--src-type", "LOCAL", "-j", string(KubeMarkMasqChain))...)
		WriteLine(natRules, append(args,
			"-m", "comment", "--comment", fmt.Sprintf(`"route LOCAL traffic for %s LB IP to service chain"`, svcNameString),
			"-m", "addrtype", "--src-type", "LOCAL", "-j", string(svcChain))...)

		// Prefer local ready endpoint chains, but fall back to ready terminating if none exist
		localEndpointChains := localReadyEndpointChains
		// TODO: uncomment once 1.22 released
		// if utilfeature.DefaultFeatureGate.Enabled(features.ProxyTerminatingEndpoints) && len(localEndpointChains) == 0 {
		// 	localEndpointChains = localServingTerminatingEndpointChains
		// }

		numLocalEndpoints := len(localEndpointChains)
		if numLocalEndpoints == 0 {
			// Blackhole all traffic since there are no local endpoints
			args = append(args[:0],
				"-A", string(svcXlbChain),
				"-m", "comment", "--comment",
				fmt.Sprintf(`"%s has no local endpoints"`, svcNameString),
				"-j",
				string(KubeMarkDropChain),
			)
			WriteLine(natRules, args...)
		} else {
			// First write session affinity rules only over local endpoints, if applicable.
			if svcInfo.SessionAffinityType() == v1.ServiceAffinityClientIP {
				for _, endpointChain := range localEndpointChains {
					WriteLine(natRules,
						"-A", string(svcXlbChain),
						"-m", "comment", "--comment", svcNameString,
						"-m", "recent", "--name", string(endpointChain),
						"--rcheck", "--seconds", strconv.Itoa(svcInfo.StickyMaxAgeSeconds()), "--reap",
						"-j", string(endpointChain))
				}
			}

			// Setup probability filter rules only over local endpoints
			for i, endpointChain := range localEndpointChains {
				// Balancing rules in the per-service chain.
				args = append(args[:0],
					"-A", string(svcXlbChain),
					"-m", "comment", "--comment",
					fmt.Sprintf(`"Balancing rule %d for %s"`, i, svcNameString),
				)
				if i < (numLocalEndpoints - 1) {
					// Each rule is a probabilistic match.
					args = append(args,
						"-m", "statistic",
						"--mode", "random",
						"--probability", probability(numLocalEndpoints-i))
				}
				// The final (or only if n == 1) rule is a guaranteed match.
				args = append(args, "-j", string(endpointChain))
				WriteLine(natRules, args...)
			}
		}
	}

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
			WriteBytesLine(natChains, existingNATChains[chain])
			WriteLine(natRules, "-X", chainString)
		}
	}

	// Finally, tail-call to the nodeports chain.  This needs to be after all
	// other service portal rules.
	// isIPv6 := proxier.iptables.IsIPv6()
	//TODO: for now harcoding to ipv4,need to see how to handle v4,v6 and dual.
	isIPv6 := false
	for address := range nodeAddresses {
		// TODO(thockin, m1093782566): If/when we have dual-stack support we will want to distinguish v4 from v6 zero-CIDRs.
		if IsZeroCIDR(address) {
			args = append(args[:0],
				"-A", string(kubeServicesChain),
				"-m", "comment", "--comment", `"kubernetes service nodeports; NOTE: this must be the last rule in this chain"`,
				"-m", "addrtype", "--dst-type", "LOCAL",
				"-j", string(kubeNodePortsChain))
			WriteLine(natRules, args...)
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
		WriteLine(natRules, args...)
	}

	// Drop the packets in INVALID state, which would potentially cause
	// unexpected connection reset.
	// https://github.com/kubernetes/kubernetes/issues/74839
	WriteLine(filterRules,
		"-A", string(kubeForwardChain),
		"-m", "conntrack",
		"--ctstate", "INVALID",
		"-j", "DROP",
	)

	// If the masqueradeMark has been added then we want to forward that same
	// traffic, this allows NodePort traffic to be forwarded even if the default
	// FORWARD policy is not accept.
	WriteLine(filterRules,
		"-A", string(kubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding rules"`,
		"-m", "mark", "--mark", fmt.Sprintf("%s/%s", masqueradeMark, masqueradeMark),
		"-j", "ACCEPT",
	)

	// The following two rules ensure the traffic after the initial packet
	// accepted by the "kubernetes forwarding rules" rule above will be
	// accepted.
	WriteLine(filterRules,
		"-A", string(kubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding conntrack pod source rule"`,
		"-m", "conntrack",
		"--ctstate", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	)
	WriteLine(filterRules,
		"-A", string(kubeForwardChain),
		"-m", "comment", "--comment", `"kubernetes forwarding conntrack pod destination rule"`,
		"-m", "conntrack",
		"--ctstate", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	)

	// Write the end-of-table markers.
	WriteLine(filterRules, "COMMIT")
	WriteLine(natRules, "COMMIT")

	// Sync rules.
	// NOTE: NoFlushTables is used so we don't flush non-kubernetes chains in the table
	iptablesData.Reset()
	iptablesData.Write(filterChains.Bytes())
	iptablesData.Write(filterRules.Bytes())
	iptablesData.Write(natChains.Bytes())
	iptablesData.Write(natRules.Bytes())

	numberFilterIptablesRules := CountBytesLines(filterRules.Bytes())
	metrics.IptablesRulesTotal.WithLabelValues(string(TableFilter)).Set(float64(numberFilterIptablesRules))
	numberNatIptablesRules := CountBytesLines(natRules.Bytes())
	metrics.IptablesRulesTotal.WithLabelValues(string(TableNAT)).Set(float64(numberNatIptablesRules))

	klog.V(5).InfoS("Restoring iptables", "rules", iptablesData.Bytes())
	err = iptInterface.RestoreAll(iptablesData.Bytes(), NoFlushTables, RestoreCounters)
	if err != nil {
		klog.ErrorS(err, "Failed to execute iptables-restore")
		metrics.IptablesRestoreFailuresTotal.Inc()
		// Revert new local ports.
		klog.V(2).InfoS("Closing local ports after iptables-restore failure")
		RevertPorts(replacementPortsMap, portsMap)
		return
	}
	//TODO: we dont have any retry logic as in kubeproxy,need to think.
	// success = true

	for name, lastChangeTriggerTimes := range endpointUpdateResult.LastChangeTriggerTimes {
		for _, lastChangeTriggerTime := range lastChangeTriggerTimes {
			latency := metrics.SinceInSeconds(lastChangeTriggerTime)
			metrics.NetworkProgrammingLatency.Observe(latency)
			klog.V(4).InfoS("Network programming", "endpoint", klog.KRef(name.Namespace, name.Name), "elapsed", latency)
		}
	}

	// Close old local ports and save new ones.
	for k, v := range portsMap {
		if replacementPortsMap[k] == nil {
			v.Close()
		}
	}
	portsMap = replacementPortsMap

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
func appendServiceCommentLocked(args []string, svcName string) []string {
	// Not printing these comments, can reduce size of iptables (in case of large
	// number of endpoints) even by 40%+. So if total number of endpoint chains
	// is large enough, we simply drop those comments.
	if endpointChainsNumber > endpointChainsNumberThreshold {
		return args
	}
	return append(args, "-m", "comment", "--comment", svcName)
}

// This assumes proxier.mu is held
func probability(n int) string {
	if n >= len(precomputedProbabilities) {
		precomputeProbabilities(n)
	}
	return precomputedProbabilities[n]
}

// This assumes proxier.mu is held
func precomputeProbabilities(numberOfPrecomputed int) {
	if len(precomputedProbabilities) == 0 {
		precomputedProbabilities = append(precomputedProbabilities, "<bad value>")
	}
	for i := len(precomputedProbabilities); i <= numberOfPrecomputed; i++ {
		precomputedProbabilities = append(precomputedProbabilities, computeProbability(i))
	}
}

func computeProbability(n int) string {
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

