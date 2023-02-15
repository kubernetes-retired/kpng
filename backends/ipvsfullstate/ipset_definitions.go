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
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/ipsets"
)

var (
	kubeLoopBackIPSetComment = "Kubernetes endpoints dst ip:port, source ip for solving hairpin purpose"
	kubeLoopBackIPSet        = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-LOOP-BACK",
		v1.IPv6Protocol: "KUBE-6-LOOP-BACK",
	}

	kubeClusterIPSetComment = "Kubernetes service cluster ip + port for masquerade purpose"
	kubeClusterIPSet        = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-CLUSTER-IP",
		v1.IPv6Protocol: "KUBE-6-CLUSTER-IP",
	}

	kubeExternalIPSetComment = "Kubernetes service external ip + port for masquerade and filter purpose"
	kubeExternalIPSet        = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-EXTERNAL-IP",
		v1.IPv6Protocol: "KUBE-6-EXTERNAL-IP",
	}

	kubeExternalIPLocalSetComment = "Kubernetes service external ip + port with externalTrafficPolicy=local"
	kubeExternalIPLocalIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-EXTERNAL-IP-LOCAL",
		v1.IPv6Protocol: "KUBE-6-EXTERNAL-IP-LOCAL",
	}

	kubeLoadBalancerSetComment = "Kubernetes service lb portal"
	kubeLoadBalancerIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-LOAD-BALANCER",
		v1.IPv6Protocol: "KUBE-6-LOAD-BALANCER",
	}

	kubeLoadBalancerLocalSetComment = "Kubernetes service load balancer ip + port with externalTrafficPolicy=local"
	kubeLoadBalancerLocalIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-LOAD-BALANCER-LOCAL",
		v1.IPv6Protocol: "KUBE-6-LOAD-BALANCER-LOCAL",
	}

	kubeLoadbalancerFWSetComment = "Kubernetes service load balancer ip + port for load balancer with sourceRange"
	kubeLoadbalancerFWIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-LOAD-BALANCER-FW",
		v1.IPv6Protocol: "KUBE-6-LOAD-BALANCER-FW",
	}

	kubeLoadBalancerSourceIPSetComment = "Kubernetes service load balancer ip + port + source IP for packet filter purpose"
	kubeLoadBalancerSourceIPSet        = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-LOAD-BALANCER-SOURCE-IP",
		v1.IPv6Protocol: "KUBE-6-LOAD-BALANCER-SOURCE-IP",
	}

	kubeLoadBalancerSourceCIDRSetComment = "Kubernetes service load balancer ip + port + source cidr for packet filter purpose"
	kubeLoadBalancerSourceCIDRIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-LOAD-BALANCER-SRC-CIDR",
		v1.IPv6Protocol: "KUBE-6-LOAD-BALANCER-SRC-CIDR",
	}

	kubeNodePortSetTCPComment = "Kubernetes nodeport TCP port for masquerade purpose"
	kubeNodePortTCPIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-NODE-PORT-TCP",
		v1.IPv6Protocol: "KUBE-6-NODE-PORT-TCP",
	}

	kubeNodePortLocalSetTCPComment = "Kubernetes nodeport TCP port with externalTrafficPolicy=local"
	kubeNodePortLocalTCPIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-NODE-PORT-LOCAL-TCP",
		v1.IPv6Protocol: "KUBE-6-NODE-PORT-LOCAL-TCP",
	}

	kubeNodePortSetUDPComment = "Kubernetes nodeport UDP port for masquerade purpose"
	kubeNodePortUDPIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-NODE-PORT-UDP",
		v1.IPv6Protocol: "KUBE-6-NODE-PORT-UDP",
	}

	kubeNodePortLocalSetUDPComment = "Kubernetes nodeport UDP port with externalTrafficPolicy=local"
	kubeNodePortLocalUDPIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-NODE-PORT-LOCAL-UDP",
		v1.IPv6Protocol: "KUBE-6-NODE-PORT-LOCAL-UDP",
	}

	kubeNodePortSetSCTPComment = "Kubernetes nodeport SCTP port for masquerade purpose with type 'hash ip:port'"
	kubeNodePortSCTPIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-NODE-PORT-SCTP",
		v1.IPv6Protocol: "KUBE-6-NODE-PORT-SCTP",
	}

	kubeNodePortLocalSetSCTPComment = "Kubernetes nodeport SCTP port with externalTrafficPolicy=local with type 'hash ip:port'"
	kubeNodePortLocalSCTPIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-NODE-PORT-LOCAL-SCTP",
		v1.IPv6Protocol: "KUBE-6-NODE-PORT-LOCAL-SCTP",
	}

	kubeHealthCheckNodePortSetComment = "Kubernetes health check node port"
	kubeHealthCheckNodePortIPSet      = map[v1.IPFamily]string{
		v1.IPv4Protocol: "KUBE-HEALTH-CHECK-NODE-PORT",
		v1.IPv6Protocol: "KUBE-6-HEALTH-CHECK-NODE-PORT",
	}
)

// ipsetInfo is all ipset we needed in ipvs
var ipsetInfo = []struct {
	name    map[v1.IPFamily]string
	setType ipsets.SetType
	comment string
}{
	{kubeLoopBackIPSet, ipsets.HashIPPortIP, kubeLoopBackIPSetComment},
	{kubeClusterIPSet, ipsets.HashIPPort, kubeClusterIPSetComment},
	{kubeExternalIPSet, ipsets.HashIPPort, kubeExternalIPSetComment},
	{kubeExternalIPLocalIPSet, ipsets.HashIPPort, kubeExternalIPLocalSetComment},
	{kubeLoadBalancerIPSet, ipsets.HashIPPort, kubeLoadBalancerSetComment},
	{kubeLoadbalancerFWIPSet, ipsets.HashIPPort, kubeLoadbalancerFWSetComment},
	{kubeLoadBalancerLocalIPSet, ipsets.HashIPPort, kubeLoadBalancerLocalSetComment},
	{kubeLoadBalancerSourceIPSet, ipsets.HashIPPortIP, kubeLoadBalancerSourceIPSetComment},
	{kubeLoadBalancerSourceCIDRIPSet, ipsets.HashIPPortNet, kubeLoadBalancerSourceCIDRSetComment},
	{kubeNodePortTCPIPSet, ipsets.BitmapPort, kubeNodePortSetTCPComment},
	{kubeNodePortLocalTCPIPSet, ipsets.BitmapPort, kubeNodePortLocalSetTCPComment},
	{kubeNodePortUDPIPSet, ipsets.BitmapPort, kubeNodePortSetUDPComment},
	{kubeNodePortLocalUDPIPSet, ipsets.BitmapPort, kubeNodePortLocalSetUDPComment},
	{kubeNodePortSCTPIPSet, ipsets.HashIPPort, kubeNodePortSetSCTPComment},
	{kubeNodePortLocalSCTPIPSet, ipsets.HashIPPort, kubeNodePortLocalSetSCTPComment},
	{kubeHealthCheckNodePortIPSet, ipsets.BitmapPort, kubeHealthCheckNodePortSetComment},
}
