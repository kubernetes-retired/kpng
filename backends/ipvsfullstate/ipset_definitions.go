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
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/ipsets"
)

const (
	kubeLoopBackIPSetComment = "Kubernetes endpoints dst ip:port, source ip for solving hairpin purpose"
	kubeLoopBackIPSet        = "KUBE-LOOP-BACK"

	kubeClusterIPSetComment = "Kubernetes service cluster ip + port for masquerade purpose"
	kubeClusterIPSet        = "KUBE-CLUSTER-IP"

	kubeExternalIPSetComment = "Kubernetes service external ip + port for masquerade and filter purpose"
	kubeExternalIPSet        = "KUBE-EXTERNAL-IP"

	kubeExternalIPLocalSetComment = "Kubernetes service external ip + port with externalTrafficPolicy=local"
	kubeExternalIPLocalSet        = "KUBE-EXTERNAL-IP-LOCAL"

	kubeLoadBalancerSetComment = "Kubernetes service lb portal"
	kubeLoadBalancerSet        = "KUBE-LOAD-BALANCER"

	kubeLoadBalancerLocalSetComment = "Kubernetes service load balancer ip + port with externalTrafficPolicy=local"
	kubeLoadBalancerLocalSet        = "KUBE-LOAD-BALANCER-LOCAL"

	kubeLoadbalancerFWSetComment = "Kubernetes service load balancer ip + port for load balancer with sourceRange"
	kubeLoadbalancerFWSet        = "KUBE-LOAD-BALANCER-FW"

	kubeLoadBalancerSourceIPSetComment = "Kubernetes service load balancer ip + port + source IP for packet filter purpose"
	kubeLoadBalancerSourceIPSet        = "KUBE-LOAD-BALANCER-SOURCE-IP"

	kubeLoadBalancerSourceCIDRSetComment = "Kubernetes service load balancer ip + port + source cidr for packet filter purpose"
	kubeLoadBalancerSourceCIDRSet        = "KUBE-LOAD-BALANCER-SOURCE-CIDR"

	kubeNodePortSetTCPComment = "Kubernetes nodeport TCP port for masquerade purpose"
	kubeNodePortSetTCP        = "KUBE-NODE-PORT-TCP"

	kubeNodePortLocalSetTCPComment = "Kubernetes nodeport TCP port with externalTrafficPolicy=local"
	kubeNodePortLocalSetTCP        = "KUBE-NODE-PORT-LOCAL-TCP"

	kubeNodePortSetUDPComment = "Kubernetes nodeport UDP port for masquerade purpose"
	kubeNodePortSetUDP        = "KUBE-NODE-PORT-UDP"

	kubeNodePortLocalSetUDPComment = "Kubernetes nodeport UDP port with externalTrafficPolicy=local"
	kubeNodePortLocalSetUDP        = "KUBE-NODE-PORT-LOCAL-UDP"

	kubeNodePortSetSCTPComment = "Kubernetes nodeport SCTP port for masquerade purpose with type 'hash ip:port'"
	kubeNodePortSetSCTP        = "KUBE-NODE-PORT-SCTP-HASH"

	kubeNodePortLocalSetSCTPComment = "Kubernetes nodeport SCTP port with externalTrafficPolicy=local with type 'hash ip:port'"
	kubeNodePortLocalSetSCTP        = "KUBE-NODE-PORT-LOCAL-SCTP-HASH"

	kubeHealthCheckNodePortSetComment = "Kubernetes health check node port"
	kubeHealthCheckNodePortSet        = "KUBE-HEALTH-CHECK-NODE-PORT"
)

// ipsetInfo is all ipset we needed in ipvs proxier
var ipsetInfo = []struct {
	name    string
	setType ipsets.SetType
	comment string
}{
	{kubeLoopBackIPSet, ipsets.HashIPPortIP, kubeLoopBackIPSetComment},
	{kubeClusterIPSet, ipsets.HashIPPort, kubeClusterIPSetComment},
	{kubeExternalIPSet, ipsets.HashIPPort, kubeExternalIPSetComment},
	{kubeExternalIPLocalSet, ipsets.HashIPPort, kubeExternalIPLocalSetComment},
	{kubeLoadBalancerSet, ipsets.HashIPPort, kubeLoadBalancerSetComment},
	{kubeLoadbalancerFWSet, ipsets.HashIPPort, kubeLoadbalancerFWSetComment},
	{kubeLoadBalancerLocalSet, ipsets.HashIPPort, kubeLoadBalancerLocalSetComment},
	{kubeLoadBalancerSourceIPSet, ipsets.HashIPPortIP, kubeLoadBalancerSourceIPSetComment},
	{kubeLoadBalancerSourceCIDRSet, ipsets.HashIPPortNet, kubeLoadBalancerSourceCIDRSetComment},
	{kubeNodePortSetTCP, ipsets.BitmapPort, kubeNodePortSetTCPComment},
	{kubeNodePortLocalSetTCP, ipsets.BitmapPort, kubeNodePortLocalSetTCPComment},
	{kubeNodePortSetUDP, ipsets.BitmapPort, kubeNodePortSetUDPComment},
	{kubeNodePortLocalSetUDP, ipsets.BitmapPort, kubeNodePortLocalSetUDPComment},
	{kubeNodePortSetSCTP, ipsets.HashIPPort, kubeNodePortSetSCTPComment},
	{kubeNodePortLocalSetSCTP, ipsets.HashIPPort, kubeNodePortLocalSetSCTPComment},
	{kubeHealthCheckNodePortSet, ipsets.BitmapPort, kubeHealthCheckNodePortSetComment},
}
