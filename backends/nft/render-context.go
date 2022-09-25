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

package nft

import (
	"bytes"
	"net"
	"strconv"
	"strings"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

const (
	// nft fragment to match a packet going to a local address
	mDAddrLocal = "fib daddr type local "
)

type renderContext struct {
	table        *nftable
	ipMask       net.IPMask
	clusterCIDRs []string

	// buffer for misc rendering to avoid multiple allocations
	buf              *bytes.Buffer
	epSeen           map[string]bool
	epCount          int
	chainNets        map[string]bool
	mapOffsets       []uint64
	localEndpointIPs []string
}

func newRenderContext(table *nftable, clusterCIDRs []string, ipMask net.IPMask) *renderContext {
	return &renderContext{
		table:        table,
		ipMask:       ipMask,
		clusterCIDRs: clusterCIDRs,

		buf:              new(bytes.Buffer),
		epSeen:           make(map[string]bool),
		chainNets:        make(map[string]bool),
		mapOffsets:       make([]uint64, *mapsCount),
		localEndpointIPs: make([]string, 0, 256),
	}
}

type EpIP struct {
	IP       string
	Endpoint *localnetv1.Endpoint
}

func (ctx *renderContext) addServiceEndpoints(serviceEndpoints *fullstate.ServiceEndpoints) {
	const daddrLocal = "fib daddr type local "

	table := ctx.table
	//iptype := table.nftIPType()

	svc := serviceEndpoints.Service
	endpoints := serviceEndpoints.Endpoints

	// write endpoint chains
	endpointIPs := ctx.epIPs(endpoints)
	ctx.epCount += len(endpointIPs)

	_, dnatChainName, filterChainName := ctx.svcChainNames(svc)

	dnatChain := ctx.table.Chains.Get(dnatChainName)
	for _, epIP := range endpointIPs {
		ctx.addEndpointChain(svc, epIP, dnatChain)
	}

	// write service chain(s)
	ctx.addSvcChain(svc, endpointIPs)

	// add the service IPs to the dispatch
	clusterIPs := &localnetv1.IPSet{}
	allSvcIPs := &localnetv1.IPSet{}

	if svc.IPs.ClusterIPs != nil {
		clusterIPs.AddSet(svc.IPs.ClusterIPs)
		allSvcIPs.AddSet(svc.IPs.ClusterIPs)
	}
	allSvcIPs.AddSet(svc.IPs.ExternalIPs)

	ips := table.IPsFromSet(allSvcIPs)

	if len(ips) == 0 {
		// nothing to contribute to the dispatch vmaps
		return
	}

	for _, i := range []struct {
		suffix, target string
	}{
		{"_dnat", dnatChainName},
		{"_filter", filterChainName},
	} {
		if ctx.table.Chains.Get(i.target).Len() == 0 {
			continue
		}

		vmapItem := ctx.table.Chains.GetItem("z_dispatch_svc" + i.suffix)
		vmap := vmapItem.Value()

		first := false
		if vmap.Len() == 0 {
			// first time here
			vmap.WriteString("  " + ctx.table.Family + " daddr vmap {\n    ")
			vmapItem.Defer(func(vmap *Leaf) {
				vmap.WriteString(" }\n")
			})
			first = true
		}

		for idx, ip := range ips {
			if first {
				first = false
			} else if idx%5 == 0 {
				vmap.WriteString(",\n    ")
			} else {
				vmap.WriteString(", ")
			}

			vmap.WriteString(ip)
			vmap.WriteString(": jump ")
			vmap.WriteString(i.target)
		}
	}
}

func (ctx *renderContext) Finalize() {
	ctx.table.RunDeferred()
	addDispatchChains(ctx.table)
	addPostroutingChain(ctx.table, ctx.clusterCIDRs, ctx.localEndpointIPs)
	ctx.table.Done()
}

func (ctx *renderContext) epIPs(endpoints []*localnetv1.Endpoint) (endpointIPs []EpIP) {
	endpointIPs = make([]EpIP, 0, len(endpoints))
	for _, ep := range endpoints {
		epIPs := ctx.table.IPsFromSet(ep.IPs)

		if len(epIPs) == 0 {
			continue
		}

		endpointIPs = append(endpointIPs, EpIP{
			IP:       epIPs[0],
			Endpoint: ep,
		})

		if ep.Local {
			for _, ip := range epIPs {
				if !ctx.epSeen[ip] {
					ctx.epSeen[ip] = true
					ctx.localEndpointIPs = append(ctx.localEndpointIPs, ip)
				}
			}
		}
	}
	return
}

func (ctx *renderContext) recordNodePort(port *localnetv1.PortMapping, targetChain string) {
	chain := ctx.table.Chains.Get("nodeports_dnat")
	if strings.HasSuffix(targetChain, "_filter") {
		chain = ctx.table.Chains.Get("nodeports_filter")
	}

	chain.WriteString("  ")
	chain.WriteString(protoMatch(port.Protocol))
	chain.WriteByte(' ')
	chain.WriteString(strconv.Itoa(int(port.NodePort)))
	chain.WriteString(" jump ")
	chain.WriteString(targetChain)
	chain.WriteByte('\n')
}

func (ctx *renderContext) svcChainNames(svc *localnetv1.Service) (chainPrefix, dnatChainName, filterChainName string) {
	chainPrefix = ctx.svcNftName(svc)
	dnatChainName = chainPrefix + "_dnat"
	filterChainName = chainPrefix + "_filter"
	return
}
