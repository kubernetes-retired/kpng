package iptables

import (
	"fmt"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	"sigs.k8s.io/kpng/client"

	"github.com/spf13/pflag"
)

var (
	flag = &pflag.FlagSet{}

	OnlyOutput = flag.Bool("only-output", false, "Only output the ipvsadm-restore file instead of calling ipvsadm-restore")
)
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

	// Create and link the kube chains.
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

	// example of how to cycle through the chains and stuff... slowly being replaced by the logic above...
	for serviceEndpoints := range ch {
		fmt.Println()
		svc := serviceEndpoints.Service
		//	endpoints := serviceEndpoints.Endpoints

		fmt.Println(fmt.Sprintf("%v", svc))
	}
}
