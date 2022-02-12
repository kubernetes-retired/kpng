package nft

import (
	"strconv"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

func (ctx *renderContext) svcNftName(svc *localnetv1.Service) string {
	return "svc_" + svc.Namespace + "_" + svc.Name
}

func (ctx *renderContext) addSvcVmap(vmapName string, svc *localnetv1.Service, epIPs []EpIP) {
	vmap := ctx.table.Maps.Get(vmapName)
	vmap.WriteString("  typeof numgen random mod 1 : verdict")

	if len(epIPs) == 0 {
		vmap.WriteByte('\n')
		return
	}

	for i, epIP := range epIPs {
		if i == 0 {
			vmap.WriteString("; elements = {\n    ")
		} else if i%5 == 0 {
			vmap.WriteString(",\n    ")
		} else {
			vmap.WriteString(", ")
		}
		vmap.WriteString(strconv.Itoa(nftKey(i)))
		vmap.WriteString(": jump ")
		vmap.WriteString(ctx.epChainName(svc, epIP.Endpoint))
	}

	vmap.WriteString(" }\n")
}

func (ctx *renderContext) addSvcChain(svc *localnetv1.Service, epIPs []EpIP) {
	chainPrefix, dnatChainName, filterChainName := ctx.svcChainNames(svc)

	dnatChain := ctx.table.Chains.Get(dnatChainName)
	filterChain := ctx.table.Chains.Get(filterChainName)

	// the default vmap with all endpoints
	vmapAllName := chainPrefix + "_eps"
	ctx.addSvcVmap(vmapAllName, svc, epIPs)

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

		if len(subset) != len(epIPs) {
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
				chain.WriteString(" numgen random mod ")
				chain.WriteString(strconv.Itoa(len(subset)))
				chain.WriteString(" vmap @")
				chain.WriteString(vmapName)
				chain.WriteByte('\n')
			}
		}
	}
}
