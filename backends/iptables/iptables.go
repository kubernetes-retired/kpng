package iptables

import (
	"bytes"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	"net"
	"sigs.k8s.io/kpng/client"
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"
)

var (
	flag = &pflag.FlagSet{}

	OnlyOutput = flag.Bool("only-output", false, "Only output the ipvsadm-restore file instead of calling ipvsadm-restore")
)

var mu           sync.Mutex // protects the following fields
var nodeLabels   map[string]string
// endpointsSynced, endpointSlicesSynced, and servicesSynced are set to true
// when corresponding objects are synced after startup. This is used to avoid
// updating iptables with some partial data after kube-proxy restart.
var endpointsSynced      bool
var endpointSlicesSynced bool
var servicesSynced       bool
var initialized          int32
var syncPeriod           time.Duration

// These are effectively const and do not need the mutex to be held.
var masqueradeAll  bool
var masqueradeMark string
var hostname       string
var nodeIP       net.IP
var recorder     record.EventRecorder
var serviceMap   ServiceMap
var endpointsMap EndpointsMap
// Since converting probabilities (floats) to strings is expensive
// and we are using only probabilities in the format of 1/n, we are
// precomputing some number of those and cache for future reuse.
var precomputedProbabilities []string

// The following buffers are used to reuse memory and avoid allocations
// that are significantly impacting performance.
var iptablesData             *bytes.Buffer
var existingFilterChainsData *bytes.Buffer
var filterChains             *bytes.Buffer
var filterRules              *bytes.Buffer
var natChains                *bytes.Buffer
var natRules                 *bytes.Buffer

// endpointChainsNumber is the total amount of endpointChains across all
// services that we will generate (it is computed at the beginning of
// syncProxyRules method). If that is large enough, comments in some
// iptable rules are dropped to improve performance.
var endpointChainsNumber int

// Values are as a parameter to select the interfaces where nodeport works.
var nodePortAddresses []string
// Inject for test purpose.
var networkInterfacer     NetworkInterfacer

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


// Callback receives the fullstate every time, so we can make the proxier.go functionality
// by rebuilding all the state as needed.  This is a port of the upstream kube proxy logic for iptables,
// which is very sophisticated.
func Callback(ch <-chan *client.ServiceEndpoints) {
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
	replacementPortsMap := map[LocalPort]Closeable{}

	// We are creating those slices ones here to avoid memory reallocations
	// in every loop. Note that reuse the memory, instead of doing:
	//   slice = <some new slice>
	// you should always do one of the below:
	//   slice = slice[:0] // and then append to it
	//   slice = append(slice[:0], ...)
	endpoints := make([]*endpointsInfo, 0)
	endpointChains := make([]Chain, 0)
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





	// OLD CODE
	// OLD CODE
	// OLD CODE
	// OLD CODE
	// OLD CODE
	// OLD CODE
	// OLD CODE
	//
	// example of how to cycle through the chains and stuff... slowly being replaced by the logic above...
	for serviceEndpoints := range ch {
		fmt.Println()
		svc := serviceEndpoints.Service
		//	endpoints := serviceEndpoints.Endpoints

		fmt.Println(fmt.Sprintf("%v", svc))
	}
}
