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

package iptables

import util "sigs.k8s.io/kpng/backends/iptables/util"

type iptablesJumpChain struct {
	table     util.Table
	dstChain  util.Chain
	srcChain  util.Chain
	comment   string
	extraArgs []string
}

const (
	// the services chain
	kubeServicesChain util.Chain = "KUBE-SERVICES"
	// the external services chain
	kubeExternalServicesChain util.Chain = "KUBE-EXTERNAL-SERVICES"
	// the nodeports chain
	kubeNodePortsChain util.Chain = "KUBE-NODEPORTS"
	// the kubernetes postrouting chain
	kubePostroutingChain util.Chain = "KUBE-POSTROUTING"
	// KubeMarkMasqChain is the mark-for-masquerade chain
	KubeMarkMasqChain util.Chain = "KUBE-MARK-MASQ"
	// KubeMarkDropChain is the mark-for-drop chain
	KubeMarkDropChain util.Chain = "KUBE-MARK-DROP"
	// the kubernetes forward chain
	kubeForwardChain util.Chain = "KUBE-FORWARD"
	// kube proxy canary chain is used for monitoring rule reload
	kubeProxyCanaryChain util.Chain = "KUBE-PROXY-CANARY"
)

var iptablesJumpChains = []iptablesJumpChain{
	{util.TableFilter, kubeExternalServicesChain, util.ChainInput, "kubernetes externally-visible service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
	{util.TableFilter, kubeExternalServicesChain, util.ChainForward, "kubernetes externally-visible service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
	{util.TableFilter, kubeNodePortsChain, util.ChainInput, "kubernetes health check service ports", nil},
	{util.TableFilter, kubeServicesChain, util.ChainForward, "kubernetes service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
	{util.TableFilter, kubeServicesChain, util.ChainOutput, "kubernetes service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
	{util.TableFilter, kubeForwardChain, util.ChainForward, "kubernetes forwarding rules", nil},
	{util.TableNAT, kubeServicesChain, util.ChainOutput, "kubernetes service portals", nil},
	{util.TableNAT, kubeServicesChain, util.ChainPrerouting, "kubernetes service portals", nil},
	{util.TableNAT, kubePostroutingChain, util.ChainPostrouting, "kubernetes postrouting rules", nil},
}

var iptablesEnsureChains = []struct {
	table util.Table
	chain util.Chain
}{
	{util.TableNAT, KubeMarkDropChain},
}

var iptablesCleanupOnlyChains = []iptablesJumpChain{
	// Present in kube 1.13 - 1.19. Removed by #95252 in favor of adding reject rules for incoming/forwarding packets to kubeExternalServicesChain
	{util.TableFilter, kubeServicesChain, util.ChainInput, "kubernetes service portals", []string{"-m", "conntrack", "--ctstate", "NEW"}},
}
