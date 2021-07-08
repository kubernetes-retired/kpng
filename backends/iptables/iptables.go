package iptables

import (
	"bytes"
	"fmt"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	"net"
	"sigs.k8s.io/kpng/client"
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
var nodeIP         net.IP
var recorder       record.EventRecorder

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

// Callback receives the fullstate every time, so we can make the proxier.go functionality
// by rebuilding all the state as needed.
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



	// OLD CODE
	// OLD CODE
	// OLD CODE
	// OLD CODE
	// OLD CODE
	// OLD CODE
	// OLD CODE
	// example of how to cycle through the chains and stuff... slowly being replaced by the logic above...
	for serviceEndpoints := range ch {
		fmt.Println()
		svc := serviceEndpoints.Service
		//	endpoints := serviceEndpoints.Endpoints

		fmt.Println(fmt.Sprintf("%v", svc))
	}
}
