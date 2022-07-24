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
	"strconv"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

func (ctx *renderContext) svcNftName(svc *localnetv1.Service) string {
	return "svc_" + svc.Namespace + "_" + svc.Name
}

func (ctx *renderContext) addSvcVmap(vmapName string, svc *localnetv1.Service, epIPs []EpIP) {
	vmap := ctx.table.Chains.Get(vmapName)

	if len(epIPs) == 0 {
		panic("no epIPs is not allowed")
		return
	}

	vmap.WriteString("  ")
	ctx.writeEndpointsVmap(vmap, svc, epIPs)
}

func (ctx *renderContext) addSvcChain(svc *localnetv1.Service, epIPs []EpIP) {
	chainPrefix, dnatChainName, filterChainName := ctx.svcChainNames(svc)

	dnatChain := ctx.table.Chains.Get(dnatChainName)
	filterChain := ctx.table.Chains.Get(filterChainName)

	// the default vmap with all endpoints
	vmapAllName := chainPrefix + "_eps"
	if len(epIPs) != 0 {
		ctx.addSvcVmap(vmapAllName, svc, epIPs)
	}

	// one rule per port, with handling for defined-but-not-on-every-endpoint cases (aka multi-port)
	for _, port := range svc.Ports {
		// filter endpoint based on port availability
		subset := make([]EpIP, 0, len(epIPs))
		for _, epIP := range epIPs {
			if epIP.Endpoint.PortMapping(port) == 0 {
				continue
			}
			subset = append(subset, epIP)
		}

		// select the chain and vmap to use
		chainName := dnatChainName
		chain := dnatChain
		if len(subset) == 0 {
			chainName = filterChainName
			chain = filterChain
		}

		vmapName := vmapAllName

		if len(subset) != len(epIPs) && len(subset) != 0 {
			// not defined on all endpoints, need a specific map
			vmapName = chainPrefix + "_eps_" + port.Name
			ctx.addSvcVmap(vmapName, svc, subset)
		}

		// write the rules
		for _, srcPort := range port.SrcPorts() {
			chain.WriteString("  ")
			if srcPort == port.NodePort {
				chain.WriteString(mDAddrLocal)

				// record this chain is associated to a node port
				ctx.recordNodePort(port, chainName)
			}
			chain.WriteString(protoMatch(port.Protocol))
			chain.WriteByte(' ')
			chain.WriteString(strconv.Itoa(int(srcPort)))

			if len(subset) == 0 {
				chain.WriteString(" reject\n")
			} else {
				chain.WriteString(" jump ")
				chain.WriteString(vmapName)
				chain.WriteByte('\n')
			}
		}
	}
}

func (ctx *renderContext) writeEndpointsVmap(w writer, svc *localnetv1.Service, epIPs []EpIP) {
	w.WriteString("numgen random mod ")
	w.WriteString(strconv.Itoa(len(epIPs)))
	w.WriteString(" vmap {")
	for i, epIP := range epIPs {
		if i == 0 {
			w.WriteString("\n    ")
		} else if i%5 == 0 {
			w.WriteString(",\n    ")
		} else {
			w.WriteString(", ")
		}
		w.WriteString(strconv.Itoa(nftKey(i)))
		w.WriteString(": jump ")
		w.WriteString(ctx.epChainName(svc, epIP.Endpoint))
	}
	w.WriteString(" }\n")
}
