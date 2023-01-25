/*
Copyright 2023 The Kubernetes Authors.

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

package ipvsfullsate

import (
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/ipsets"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/iptables"
)

// Setup is used for setting up the backend, initialize ipvs, ipsets and iptales.
func (b *backend) Setup() {
	var err error
	ipsetList := make(map[string]*ipsets.Set)
	controller = newController()

	// setup ipvs manager
	// this call will do all the required sysctls, and create dummy interface
	// for binding cluster ips to host interface.
	err = controller.ipvsManager.Setup()
	if err != nil {
		klog.Fatal("unable to initialize ipvs manager", "error", err)
	}

	// setup ipsets manager
	// right now it's a No-Op, will shift ipset initialization there
	err = controller.ipsetsManager.Setup()
	if err != nil {
		klog.Fatal("unable to initialize ipvs manager", "error", err)
	}

	// initialize ipsets
	for _, is := range ipsetInfo {
		set, err := controller.ipsetsManager.CreateSet(is.name, is.setType, is.comment)
		ipsetList[set.GetName()] = set
		if err != nil {
			klog.Fatal("unable to create ipset", "set", is.name, "error", err)
		}
	}

	// add custom chains to NAT table
	for _, chain := range []iptables.Chain{kubeServicesChain, KubeFireWallChain, kubePostroutingChain, KubeMarkMasqChain,
		KubeNodePortChain, KubeMarkDropChain, KubeForwardChain, KubeLoadBalancerChain} {
		controller.iptManager.AddChain(chain, iptables.TableNat)
	}

	// add custom chains to FILTER table
	for _, chain := range []iptables.Chain{KubeForwardChain, KubeNodePortChain} {
		controller.iptManager.AddChain(chain, iptables.TableFilter)
	}

	// add rules for NAT table
	for _, rule := range GetNatRules(true) {
		controller.iptManager.AddRule(rule, iptables.TableNat)
	}

	// add rules for FILTER table
	for _, rule := range GetFilterRules(true) {
		controller.iptManager.AddRule(rule, iptables.TableFilter)
	}

	// Apply will write the rules to IPTables.
	controller.iptManager.Apply()

	// run HTTP listener for proxyMode detection
	go func() {
		err = controller.SetUpHttpListen()
		if err != nil {
			return
		}
	}()
}

func (c *Controller) SetUpHttpListen() error {
	errCh := make(chan error)
	c.ServeProxyMode(errCh)
	return <-errCh
}
