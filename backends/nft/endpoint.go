package nft

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/diffstore2"
)

func (ctx *renderContext) epChainName(svc *localnetv1.Service, ep *localnetv1.Endpoint) string {
	ips := ctx.table.IPsFromSet(ep.IPs)
	ipHex := hex.EncodeToString(netip.MustParseAddr(ips[0]).AsSlice())
	return ctx.svcNftName(svc) + "_ep_" + ipHex
}

func (ctx *renderContext) addEndpointChain(svc *localnetv1.Service, epIP EpIP, svcChain *diffstore2.BufferLeaf) (epChainName string) {
	ep := epIP.Endpoint

	epChainName = ctx.epChainName(svc, ep)

	epChain := ctx.table.Chains.Get(epChainName)
	family := ctx.table.Family

	switch sa := svc.SessionAffinity.(type) {
	case *localnetv1.Service_ClientIP:
		recentSet := epChainName + "_recent"
		if recentSetV := ctx.table.Sets.Get(recentSet); recentSetV.Len() == 0 {
			recentSetV.WriteString("  typeof " + family + " daddr; flags timeout;\n")
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
