package ipvs

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	ipt "sigs.k8s.io/kpng/backends/ipvs/internal/iptables"
)

// masqueradeMark should match the mark of kubelet
const masqueradeBit = 14
const masqueradeValue = 1 << uint(masqueradeBit)

var masqueradeMark = fmt.Sprintf("%#08x", masqueradeValue)

// kubeServicesChain is the services portal chain
const kubeServicesChain = ipt.Chain("KUBE-SERVICES")

// KubeFireWallChain is the kubernetes firewall chain.
const KubeFireWallChain = ipt.Chain("KUBE-FIREWALL")

// kubePostroutingChain is the kubernetes postrouting chain.
const kubePostroutingChain = ipt.Chain("KUBE-POSTROUTING")

// KubeMarkMasqChain is the mark-for-masquerade chain.
const KubeMarkMasqChain = ipt.Chain("KUBE-MARK-MASQ")

// KubeNodePortChain is the kubernetes node port chain.
const KubeNodePortChain = ipt.Chain("KUBE-NODE-PORT")

// KubeMarkDropChain is the mark-for-drop chain.
const KubeMarkDropChain = ipt.Chain("KUBE-MARK-DROP")

// KubeForwardChain is the kubernetes forward chain.
const KubeForwardChain = ipt.Chain("KUBE-FORWARD")

// KubeLoadBalancerChain is the kubernetes chain for loadbalancer type service.
const KubeLoadBalancerChain = ipt.Chain("KUBE-LOAD-BALANCER")

func GetNatRules(supportsFullyRandomized bool, ipFamily v1.IPFamily) []ipt.Rule {

	// link default IPTables chains to custom chains where we will program the iptable rules.
	rules := []ipt.Rule{
		{
			From: ipt.ChainOutput,
			To:   kubeServicesChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        "kubernetes service portals",
				},
			},
		},
		{
			From: ipt.ChainPreRouting,
			To:   kubeServicesChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        "kubernetes service portals",
				},
			},
		},
		{
			From: ipt.ChainPostRouting,
			To:   kubePostroutingChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        "kubernetes postrouting rules",
				},
			},
		},
	}

	// build iptable rules for ipsets
	rules = append(rules,
		ipt.Rule{
			From:   kubePostroutingChain,
			Target: ipt.TargetMasquerade,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeLoopBackIPSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeLoopBackIPSet[ipFamily] + " dst,dst,src",
				},
			},
		},

		ipt.Rule{
			From: kubeServicesChain,
			To:   KubeLoadBalancerChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeLoadBalancerSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeLoadBalancerIPSet[ipFamily] + " dst,dst",
				},
			},
		},
		ipt.Rule{
			From: KubeLoadBalancerChain,
			To:   KubeFireWallChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeLoadbalancerFWSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeLoadbalancerFWIPSet[ipFamily] + " dst,dst",
				},
			},
		},
		ipt.Rule{
			From:   KubeFireWallChain,
			Target: ipt.TargetReturn,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeLoadBalancerSourceCIDRSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeLoadBalancerSourceCIDRIPSet[ipFamily] + " dst,dst,src",
				},
			},
		},
		ipt.Rule{
			From:   KubeFireWallChain,
			Target: ipt.TargetReturn,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeLoadBalancerSourceIPSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeLoadBalancerSourceIPSet[ipFamily] + " dst,dst,src",
				},
			},
		},
		ipt.Rule{
			From:   KubeLoadBalancerChain,
			Target: ipt.TargetReturn,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeLoadBalancerLocalSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeLoadBalancerLocalIPSet[ipFamily] + " dst,dst",
				},
			},
		},
		ipt.Rule{
			From:     KubeNodePortChain,
			To:       KubeMarkMasqChain,
			Protocol: ipt.ProtocolTCP,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeNodePortSetTCPComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeNodePortTCPIPSet[ipFamily] + " dst",
				},
			},
		},
		ipt.Rule{
			From:     KubeNodePortChain,
			Target:   ipt.TargetReturn,
			Protocol: ipt.ProtocolTCP,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeNodePortLocalSetTCPComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeNodePortLocalTCPIPSet[ipFamily] + " dst",
				},
			},
		},
		ipt.Rule{
			From:     KubeNodePortChain,
			To:       KubeMarkMasqChain,
			Protocol: ipt.ProtocolUDP,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeNodePortSetUDPComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeNodePortUDPIPSet[ipFamily] + " dst",
				},
			},
		},
		ipt.Rule{
			From:     KubeNodePortChain,
			Target:   ipt.TargetReturn,
			Protocol: ipt.ProtocolUDP,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeNodePortLocalSetUDPComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeNodePortLocalUDPIPSet[ipFamily] + " dst",
				},
			},
		},
		ipt.Rule{
			From:     KubeNodePortChain,
			To:       KubeMarkMasqChain,
			Protocol: ipt.ProtocolSCTP,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeNodePortSetSCTPComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeNodePortSCTPIPSet[ipFamily] + " dst,dst",
				},
			},
		},
		ipt.Rule{
			From:     KubeNodePortChain,
			Target:   ipt.TargetReturn,
			Protocol: ipt.ProtocolSCTP,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeNodePortLocalSetSCTPComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeNodePortLocalSCTPIPSet[ipFamily] + " dst,dst",
				},
			},
		})

	if *MasqueradeAll {
		rules = append(rules, ipt.Rule{
			From: kubeServicesChain,
			To:   KubeMarkMasqChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeClusterIPSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeClusterIPSet[ipFamily] + " dst,dst",
				},
			},
		})
	} else {
		// Masquerade all OUTPUT traffic coming from a service ip.
		// The kube dummy interface has all service VIPs assigned which
		// results in the service VIP being picked as the source IP to reach
		// a VIP. This leads to a connection from VIP:<random port> to
		// VIP:<service port>.
		// Always masquerading OUTPUT (node-originating) traffic with a VIP
		// source ip and service port destination fixes the outgoing connections.
		rules = append(rules, ipt.Rule{
			From: kubeServicesChain,
			To:   KubeMarkMasqChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeClusterIPSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeClusterIPSet[ipFamily] + " src,dst",
				},
			},
		})
	}

	// Build masquerade rules for packets to external IPs.
	rules = append(rules,
		ipt.Rule{
			From: kubeServicesChain,
			To:   KubeMarkMasqChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeExternalIPSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeExternalIPSet[ipFamily] + " dst,dst",
				},
			},
		},
		ipt.Rule{
			From:   kubeServicesChain,
			Target: ipt.TargetAccept,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeExternalIPSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeExternalIPSet[ipFamily] + " dst,dst",
				},
				{
					Module:       ipt.MatchModulePhysDev,
					ModuleOption: ipt.MatchModulePhysDevOptionPhysDevIsIn,
					Inverted:     true,
				},
				{
					Module:       ipt.MatchModuleAddrType,
					ModuleOption: ipt.MatchModuleAddrTypeOptionSrcType,
					Value:        "LOCAL",
					Inverted:     true,
				},
			},
		},
		ipt.Rule{
			From:   kubeServicesChain,
			Target: ipt.TargetAccept,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeExternalIPSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeExternalIPSet[ipFamily] + " dst,dst",
				},
				{
					Module:       ipt.MatchModuleAddrType,
					ModuleOption: ipt.MatchModuleAddrTypeOptionDstType,
					Value:        "LOCAL",
				},
			},
		},
	)

	// Build masquerade rules for packets to external IPs (externalTrafficPolicy=Local)
	rules = append(rules,
		ipt.Rule{
			From:   kubeServicesChain,
			Target: ipt.TargetAccept,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeExternalIPLocalSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeExternalIPLocalIPSet[ipFamily] + " dst,dst",
				},
				{
					Module:       ipt.MatchModulePhysDev,
					ModuleOption: ipt.MatchModulePhysDevOptionPhysDevIsIn,
					Inverted:     true,
				},
				{
					Module:       ipt.MatchModuleAddrType,
					ModuleOption: ipt.MatchModuleAddrTypeOptionSrcType,
					Value:        "LOCAL",
					Inverted:     true,
				},
			},
		},
		ipt.Rule{
			From:   kubeServicesChain,
			Target: ipt.TargetAccept,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeExternalIPLocalSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeExternalIPLocalIPSet[ipFamily] + " dst,dst",
				},
				{
					Module:       ipt.MatchModuleAddrType,
					ModuleOption: ipt.MatchModuleAddrTypeOptionDstType,
					Value:        "LOCAL",
				},
			},
		},
	)

	// Rules for
	rules = append(rules,
		ipt.Rule{
			From: kubeServicesChain,
			To:   KubeNodePortChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleAddrType,
					ModuleOption: ipt.MatchModuleAddrTypeOptionDstType,
					Value:        "LOCAL",
				},
			},
		})

	rules = append(rules,
		// mark drop for KUBE-LOAD-BALANCER
		ipt.Rule{
			From: KubeLoadBalancerChain,
			To:   KubeMarkMasqChain,
		},
		// mark drop for KUBE-FIRE-WALL
		ipt.Rule{
			From: KubeFireWallChain,
			To:   KubeMarkDropChain,
		},
	)

	// Accept all traffic with destination of ipvs virtual service, in case other iptables rules
	// block the traffic, that may result in ipvs rules invalid.
	// Those rules must be in the end of KUBE-SERVICE chain
	rules = append(rules,
		ipt.Rule{
			From:   kubeServicesChain,
			Target: ipt.TargetAccept,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeClusterIPSet[ipFamily] + " dst,dst",
				},
			},
		},
		ipt.Rule{
			From:   kubeServicesChain,
			Target: ipt.TargetAccept,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeLoadBalancerIPSet[ipFamily] + " dst,dst",
				},
			},
		},
	)

	// Install the kubernetes-specific postrouting rules. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	rules = append(rules,
		// NB: THIS MUST MATCH the corresponding code in the kubelet
		ipt.Rule{
			From:   kubePostroutingChain,
			Target: ipt.TargetReturn,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleMark,
					ModuleOption: ipt.MatchModuleMarkOptionMark,
					Value:        masqueradeMark + "/" + masqueradeMark,
					Inverted:     true,
				},
			},
		},
		// Clear the mark to avoid re-masquerading if the packet re-traverses the network stack.
		ipt.Rule{
			From:              kubePostroutingChain,
			Target:            ipt.TargetMark,
			TargetOption:      ipt.TargetMarkOptionXorMark,
			TargetOptionValue: masqueradeMark,
		},
	)

	if supportsFullyRandomized {
		rules = append(rules,
			ipt.Rule{
				From:         kubePostroutingChain,
				Target:       ipt.TargetMasquerade,
				TargetOption: ipt.TargetMasqueradeOptionFullyRandomized,
				MatchOptions: []ipt.MatchOption{
					{
						Module:       ipt.MatchModuleComment,
						ModuleOption: ipt.MatchModuleCommentOptionComment,
						Value:        "kubernetes service traffic requiring SNAT",
					},
				},
			})
	} else {
		rules = append(rules,
			ipt.Rule{
				From:   kubePostroutingChain,
				Target: ipt.TargetMasquerade,
				MatchOptions: []ipt.MatchOption{
					{
						Module:       ipt.MatchModuleComment,
						ModuleOption: ipt.MatchModuleCommentOptionComment,
						Value:        "kubernetes service traffic requiring SNAT",
					},
				},
			})
	}

	// Install the kubernetes-specific masquerade mark rule. We use a whole chain for
	// this so that it is easier to flush and change, for example if the mark
	// value should ever change.
	rules = append(rules,
		ipt.Rule{
			From:              KubeMarkMasqChain,
			Target:            ipt.TargetMark,
			TargetOption:      ipt.TargetMarkOptionOrMark,
			TargetOptionValue: masqueradeMark,
		})
	return rules
}

func GetFilterRules(supportsFullyRandomized bool, ipFamily v1.IPFamily) []ipt.Rule {
	// link default IPTables chains to custom chains where we will program the iptable rules.
	rules := []ipt.Rule{
		{
			From: ipt.ChainForward,
			To:   KubeForwardChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        "kubernetes forwarding rules",
				},
			},
		},
		{
			From: ipt.ChainInput,
			To:   KubeNodePortChain,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        "kubernetes health check rules",
				},
			},
		},
	}

	rules = append(rules,
		// If the masqueradeMark has been added then we want to forward that same
		// traffic, this allows NodePort traffic to be forwarded even if the default
		// FORWARD policy is not accept.
		ipt.Rule{
			From:   KubeForwardChain,
			Target: ipt.TargetAccept,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        "kubernetes forwarding rules",
				},
				{
					Module:       ipt.MatchModuleMark,
					ModuleOption: ipt.MatchModuleMarkOptionMark,
					Value:        masqueradeMark + "/" + masqueradeMark,
				},
			},
		},
		// The following rule ensures the traffic after the initial packet accepted
		// by the "kubernetes forwarding rules" rule above will be accepted.
		ipt.Rule{
			From:   KubeForwardChain,
			Target: ipt.TargetAccept,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        "kubernetes forwarding conntrack rule",
				},
				{
					Module:       ipt.MatchModuleConnTrack,
					ModuleOption: ipt.MatchModuleConnTrackOptionConnState,
					Value:        "RELATED,ESTABLISHED",
				},
			},
		},
		// Add rule to accept traffic towards health check node port.
		ipt.Rule{
			From:   KubeNodePortChain,
			Target: ipt.TargetAccept,
			MatchOptions: []ipt.MatchOption{
				{
					Module:       ipt.MatchModuleComment,
					ModuleOption: ipt.MatchModuleCommentOptionComment,
					Value:        kubeHealthCheckNodePortSetComment,
				},
				{
					Module:       ipt.MatchModuleSet,
					ModuleOption: ipt.MatchModuleSetOptionSet,
					Value:        kubeHealthCheckNodePortIPSet[ipFamily] + " dst",
				},
			},
		})
	return rules
}
