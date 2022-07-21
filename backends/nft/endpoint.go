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
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/diffstore"
)

func (ctx *renderContext) epChainName(svc *localnetv1.Service, ep *localnetv1.Endpoint) string {
	ips := ctx.table.IPsFromSet(ep.IPs)
	ipHex := hex.EncodeToString(netip.MustParseAddr(ips[0]).AsSlice())
	return ctx.svcNftName(svc) + "_ep_" + ipHex
}

func (ctx *renderContext) addEndpointChain(svc *localnetv1.Service, epIP EpIP, svcChain *diffstore.BufferLeaf) (epChainName string) {
	ep := epIP.Endpoint

	epChainName = ctx.epChainName(svc, ep)

	epChain := ctx.table.Chains.Get(epChainName)
	family := ctx.table.Family

	switch sa := svc.SessionAffinity.(type) {
	case *localnetv1.Service_ClientIP:
		recentSet := epChainName + "_recent"
		if recentSetV := ctx.table.Sets.Get(recentSet); recentSetV.Len() == 0 {
			recentSetV.WriteString("  type " + ctx.table.nftIPType() + "; flags timeout;\n")
		}

		timeout := strconv.Itoa(int(sa.ClientIP.TimeoutSeconds))
		fmt.Fprint(epChain, "  update @"+recentSet+" { "+family+" saddr timeout "+timeout+"s }\n")

		fmt.Fprint(svcChain, "  "+family+" saddr @"+recentSet+" jump "+epChainName+"\n")
	}

	for _, nodePort := range []bool{false, true} {
		for _, port := range svc.Ports {
			srcPort := port.Port
			if nodePort {
				srcPort = port.NodePort
			}
			if srcPort == 0 {
				continue
			}

			targetPort := epIP.Endpoint.PortMapping(port)
			if targetPort == 0 {
				continue
			}

			epChain.WriteString("  ")
			if nodePort {
				epChain.WriteString(mDAddrLocal)
			}
			epChain.WriteString(protoMatch(port.Protocol))
			epChain.WriteByte(' ')
			epChain.WriteString(strconv.Itoa(int(srcPort)))
			epChain.WriteString(" dnat to ")
			epChain.WriteString(epIP.IP)

			if srcPort != targetPort {
				epChain.WriteByte(':')
				epChain.WriteString(strconv.Itoa(int(targetPort)))
			}

			epChain.WriteByte('\n')

		}
	}

	return
}
