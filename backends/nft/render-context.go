package nft

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/cespare/xxhash"
	"k8s.io/klog"

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

func (ctx *renderContext) addServiceEndpoints(serviceEndpoints *fullstate.ServiceEndpoints) {
	const daddrLocal = "fib daddr type local "

	doComments := !*skipComments && bool(klog.V(1))

	table := ctx.table
	family := table.Family

	svc := serviceEndpoints.Service
	endpoints := serviceEndpoints.Endpoints

	mapH := xxhash.Sum64String(svc.Namespace+"/"+svc.Name) % (*mapsCount)
	svcOffset := ctx.mapOffsets[mapH]
	ctx.mapOffsets[mapH] += uint64(len(endpoints))

	endpointsMap := fmt.Sprintf("endpoints_%04x", mapH)

	clusterIPs := &localnetv1.IPSet{}
	allSvcIPs := &localnetv1.IPSet{}

	if svc.IPs.ClusterIPs != nil {
		clusterIPs.AddSet(svc.IPs.ClusterIPs)
		allSvcIPs.AddSet(svc.IPs.ClusterIPs)
	}
	allSvcIPs.AddSet(svc.IPs.ExternalIPs)

	ips := table.IPsFromSet(allSvcIPs)

	if len(ips) == 0 {
		return // XXX check: NodePort services can theorically have no IP but work anyway
	}

	// compute endpoints
	endpointIPs := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		epIPs := table.IPsFromSet(ep.IPs)

		if len(epIPs) == 0 {
			continue
		}

		endpointIPs = append(endpointIPs, epIPs[0])

		if ep.Local {
			for _, ip := range epIPs {
				if !ctx.epSeen[ip] {
					ctx.epSeen[ip] = true
					ctx.localEndpointIPs = append(ctx.localEndpointIPs, ip)
				}
			}
		}
	}

	ctx.epCount += len(endpointIPs)

	// filter or nat? reject does not work in prerouting
	prefix := "dnat_"
	if len(endpointIPs) == 0 {
		prefix = "filter_"
	}

	daddrMatch := family + " daddr"

	svcChainNameFragment := strings.Join([]string{"svc", svc.Namespace, svc.Name}, "_")
	svcChain := prefix + svcChainNameFragment

	rule := ctx.buf
	hasRules := false

	switch sa := svc.SessionAffinity.(type) {
	case *localnetv1.Service_ClientIP:
		rule.Reset()
		rule.WriteString("  numgen random mod ")
		rule.WriteString(strconv.Itoa(len(endpointIPs)))
		rule.WriteString(" vmap { ")

		chain := table.Chains.Get(svcChain)
		timeout := strconv.Itoa(int(sa.ClientIP.TimeoutSeconds))

		for idx, ip := range endpointIPs {
			ipHex := hex.EncodeToString(netip.MustParseAddr(ip).AsSlice())

			epSet := svcChainNameFragment + "_epset_" + ipHex
			if epSetV := table.Sets.Get(epSet); epSetV.Len() == 0 {
				fmt.Fprint(epSetV, "  typeof "+family+" daddr; flags timeout;\n")
			}

			epChainName := svcChain + "_ep_" + ipHex
			epChain := table.Chains.Get(epChainName)
			fmt.Fprint(epChain, "  update @"+epSet+" { "+family+" saddr timeout "+timeout+"s }\n")

			fmt.Fprint(chain, "  "+family+" saddr @"+epSet+" jump "+epChainName+"\n")

			for _, nodePort := range []bool{false, true} {
				for _, port := range svc.Ports {
					srcPort := port.Port
					if nodePort {
						srcPort = port.NodePort
					}
					if srcPort == 0 {
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
					epChain.WriteString(ip)

					if srcPort != port.TargetPort {
						epChain.WriteByte(':')
						epChain.WriteString(strconv.Itoa(int(port.TargetPort)))
					}

					epChain.WriteByte('\n')

				}
			}

			if idx != 0 {
				rule.WriteString(", ")
			}
			rule.WriteString(strconv.Itoa(nftKey(idx)))
			rule.WriteString(": jump ")
			rule.WriteString(epChainName)

			hasRules = true
		}

		if hasRules {
			rule.WriteString("}\n")
			rule.WriteTo(chain)
		}

	default:
		// add endpoints to the map
		if len(endpointIPs) != 0 {
			item := table.Maps.GetItem(endpointsMap)
			epMap := item.Value()

			if epMap.Len() == 0 {
				epMap.WriteString("  typeof numgen random mod 1 : ")
				epMap.WriteString(family)
				epMap.WriteString(" daddr\n")
				epMap.WriteString("  elements = {")
				item.Defer(func(m *Leaf) { m.WriteString("}\n") })
			} else {
				epMap.WriteString(", ")
			}

			if doComments {
				fmt.Fprintf(epMap, "\\\n    # %s/%s", svc.Namespace, svc.Name)
			}

			fmt.Fprint(epMap, "\\\n    ")
			for idx, ip := range endpointIPs {
				if idx != 0 {
					epMap.WriteString(", ")
				}
				key := nftKey(int(svcOffset) + idx)

				epMap.WriteString(strconv.Itoa(key))
				epMap.WriteString(" : ")
				epMap.WriteString(ip)
			}
		}

		for _, protocol := range []localnetv1.Protocol{
			localnetv1.Protocol_TCP,
			localnetv1.Protocol_UDP,
			localnetv1.Protocol_SCTP,
		} {
			rule.Reset()

			// build the rule
			ruleSpec := dnatRule{
				Namespace:   svc.Namespace,
				Name:        svc.Name,
				Protocol:    protocol,
				Ports:       svc.Ports,
				EndpointIPs: endpointIPs,
			}

			// handle standard dnat
			ruleSpec.WriteTo(rule, false, endpointsMap, svcOffset)

			if rule.Len() != 0 {
				rule.WriteTo(table.Chains.Get(svcChain))
				hasRules = true
			}

			// handle node ports
			rule.Reset()
			ruleSpec.WriteTo(rule, true, endpointsMap, svcOffset)

			if rule.Len() != 0 {
				rule.WriteTo(table.Chains.Get(svcChain))
				hasRules = true
			}
		}
	}

	if !hasRules {
		return // no rules, no refs to make
	}

	for _, port := range svc.Ports {
		srcPort := port.NodePort
		if srcPort == 0 {
			continue
		}

		nodeports := table.Chains.Get("nodeports")
		nodeports.WriteString("  ")
		nodeports.WriteString(protoMatch(port.Protocol))
		nodeports.WriteByte(' ')
		nodeports.WriteString(strconv.Itoa(int(srcPort)))
		nodeports.WriteString(" jump ")
		nodeports.WriteString(svcChain)
		nodeports.WriteByte('\n')
	}

	// dispatch group chain (ie: dnat_net_0a002700 for 10.0.39.x and a /24 mask)
	familyClusterIPs := table.IPsFromSet(clusterIPs)

	if len(familyClusterIPs) != 0 {
		// this family owns the cluster IP => build the dispatch chain
		mask := ctx.ipMask

		for _, ipStr := range familyClusterIPs {
			ip := net.ParseIP(ipStr).Mask(mask)

			// get the dispatch chain
			chain := prefix + "net_" + hex.EncodeToString(ip)

			// add service chain in dispatch
			vmapAdd(table.Chains.GetItem(chain), family+" daddr", ipStr+": jump "+svcChain)

			// reference the dispatch chain from the global dispatch (of not already done) (ie: z_dnat_all)
			if !ctx.chainNets[chain] {
				ipNet := &net.IPNet{
					IP:   ip,
					Mask: mask,
				}

				vmapAdd(table.Chains.GetItem("z_"+prefix+"all"), daddrMatch, ipNet.String()+": jump "+chain)

				ctx.chainNets[chain] = true
			}
		}
	}

	// handle external IPs dispatch
	extIPsSet := svc.IPs.AllIngress()

	extIPs := table.IPsFromSet(extIPsSet)

	if len(extIPs) != 0 {
		extChain := table.Chains.GetItem(prefix + "external")
		for _, extIP := range extIPs {
			// XXX should this be by protocol and port to allow external IP mutualization between services?
			vmapAdd(extChain, daddrMatch, extIP+": jump "+svcChain)
		}
	}
}

func (ctx *renderContext) Finalize() {
	ctx.table.RunDeferred()
	addDispatchChains(ctx.table)
	addPostroutingChain(ctx.table, ctx.clusterCIDRs, ctx.localEndpointIPs)
	ctx.table.Done()
}
