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

	"github.com/vishvananda/netlink"
	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/kpng/api/localnetv1"
	iptablesutil "sigs.k8s.io/kpng/backends/iptables/util"
	"sigs.k8s.io/kpng/backends/ipvs-as-sink/exec"
	util "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
	"sigs.k8s.io/kpng/client/lightdiffstore"
)

type proxier struct {
	ipFamily v1.IPFamily

	dryRun           bool
	nodeAddresses    []string
	schedulingMethod string
	weight           int32
	masqueradeMark   string
	masqueradeAll    bool

	dummy netlink.Link

	iptables util.IPTableInterface
	ipset    util.Interface
	exec     exec.Interface
	//ipvs           util.Interface
	//localDetector  proxyutiliptables.LocalTrafficDetector
	//portMapper     netutils.PortOpener
	//recorder       events.EventRecorder
	//serviceHealthServer healthcheck.ServiceHealthServer
	//healthzServer       healthcheck.ProxierHealthUpdater

	// <namespace>/<service-name>/<endpoint key>/<ip> -> <ip>
	endpoints *lightdiffstore.DiffStore
	// <namespace>/<service-name>/<ip>/<protocol>:<port> -> ipvsLB
	servicePorts *lightdiffstore.DiffStore

	ipsetList map[string]*IPSet
	//servicePortMap map[string]map[string]*BaseServicePortInfo
	portMap map[string]map[string]localnetv1.PortMapping
	// The following buffers are used to reuse memory and avoid allocations
	// that are significantly impacting performance.
	iptablesData     *bytes.Buffer
	filterChainsData *bytes.Buffer
	natChains        iptablesutil.LineBuffer
	filterChains     iptablesutil.LineBuffer
	natRules         iptablesutil.LineBuffer
	filterRules      iptablesutil.LineBuffer
}

func NewProxier(ipFamily v1.IPFamily,
	dummy netlink.Link,
	ipsetInterface util.Interface,
	iptInterface util.IPTableInterface,
	nodeIPs []string,
	schedulingMethod, masqueradeMark string,
	masqueradeAll bool,
	weight int32) *proxier {
	return &proxier{
		ipFamily:         ipFamily,
		dummy:            dummy,
		nodeAddresses:    nodeIPs,
		schedulingMethod: schedulingMethod,
		weight:           weight,
		ipset:            ipsetInterface,
		iptables:         iptInterface,
		masqueradeMark:   masqueradeMark,
		masqueradeAll:    masqueradeAll,
		ipsetList:        make(map[string]*IPSet),
		portMap:          make(map[string]map[string]localnetv1.PortMapping),
		endpoints:        lightdiffstore.New(),
		servicePorts:     lightdiffstore.New(),
		iptablesData:     bytes.NewBuffer(nil),
		filterChainsData: bytes.NewBuffer(nil),
		natChains:        iptablesutil.LineBuffer{},
		natRules:         iptablesutil.LineBuffer{},
		filterChains:     iptablesutil.LineBuffer{},
		filterRules:      iptablesutil.LineBuffer{},
	}
}

func (p *proxier) initializeIPSets() {
	// initialize ipsetList with all sets we needed
	for _, is := range ipsetInfo {
		p.ipsetList[is.name] = newIPSet(p.ipset, is.name, is.setType, p.ipFamily, is.comment)
	}

	// make sure ip sets exists in the system.
	for _, set := range p.ipsetList {
		if err := ensureIPSet(set); err != nil {
			return
		}
	}
}
