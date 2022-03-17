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
	"fmt"
	"net"
	"os"
	"strings"

	"sigs.k8s.io/kpng/backends/ipvs-as-sink/config"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/klog"

	"time"

	"sigs.k8s.io/kpng/backends/ipvs-as-sink/exec"
	"sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
	"sigs.k8s.io/kpng/client/serviceevents"

	"github.com/google/seesaw/ipvs"
	"github.com/vishvananda/netlink"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"

	netutils "k8s.io/utils/net"
)

// In IPVS proxy mode, the following flags need to be set
const (
	sysctlBridgeCallIPTables           = "net/bridge/bridge-nf-call-iptables"
	sysctlVSConnTrack                  = "net/ipv4/vs/conntrack"
	sysctlConnReuse                    = "net/ipv4/vs/conn_reuse_mode"
	sysctlExpireNoDestConn             = "net/ipv4/vs/expire_nodest_conn"
	sysctlExpireQuiescentTemplate      = "net/ipv4/vs/expire_quiescent_template"
	sysctlForward                      = "net/ipv4/ip_forward"
	sysctlArpIgnore                    = "net/ipv4/conf/all/arp_ignore"
	sysctlArpAnnounce                  = "net/ipv4/conf/all/arp_announce"
	connReuseMinSupportedKernelVersion = "4.1"
	// https://github.com/torvalds/linux/commit/35dfb013149f74c2be1ff9c78f14e6a3cd1539d1
	connReuseFixedKernelVersion = "5.9"
)

func init() {
	backendcmd.Register("to-ipvs", func() backendcmd.Cmd { return New() })
}

type Backend struct {
	localsink.Config
	svcs     map[string]*localnetv1.Service
	proxiers map[v1.IPFamily]*proxier
	svcEPMap map[string]int

	dryRun           bool
	nodeAddresses    []string
	schedulingMethod string
	weight           int32

	dummy netlink.Link

	masqueradeAll bool

	k8sProxyConfig *config.KpngConfiguration
}

var _ decoder.Interface = &Backend{}

func New() *Backend {
	return &Backend{
		proxiers:       make(map[v1.IPFamily]*proxier),
		svcs:           map[string]*localnetv1.Service{},
		svcEPMap:       map[string]int{},
		k8sProxyConfig: new(config.KpngConfiguration),
	}
}

func (s *Backend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(serviceevents.Wrap(s)))
}

// ------------------------------------------------------------------------
// (IP, port) listener interface
//
var _ serviceevents.IPPortsListener = &Backend{}

func (s *Backend) AddIPPort(svc *localnetv1.Service, ip string, IPKind serviceevents.IPKind, port *localnetv1.PortMapping) {
	klog.V(2).Infof("AddIPPort (svc: %v, svc-ip: %v, port: %v)", svc, ip, port)
	serviceKey := getServiceKey(svc)
	s.svcs[serviceKey] = svc
	if svc.Type == ClusterIPService {
		s.handleClusterIPService(svc, ip, IPKind, port)
	}

	if svc.Type == NodePortService {
		s.handleNodePortService(svc, ip, port)
	}

	if svc.Type == LoadBalancerService {
		s.handleLbService(svc, ip, IPKind, port)
	}
}

func (s *Backend) DeleteIPPort(svc *localnetv1.Service, ip string, IPKind serviceevents.IPKind, port *localnetv1.PortMapping) {
	klog.V(2).Infof("DeleteIPPort (svc: %v, svc-ip: %v, port: %v)", svc, ip, port)
	if svc.Type == ClusterIPService {
		s.deleteClusterIPService(svc, ip, IPKind, port)
	}

	if svc.Type == NodePortService {
		s.deleteNodePortService(svc, ip, port)
	}

	if svc.Type == LoadBalancerService {
		s.deleteLbService(svc, ip, IPKind, port)
	}
}

// ------------------------------------------------------------------------
// IP listener interface
//
var _ serviceevents.IPsListener = &Backend{}

func (s *Backend) AddIP(svc *localnetv1.Service, ip string, ipKind serviceevents.IPKind) {
	klog.V(2).Infof("AddIP (svc: %v, svc-ip: %v, type: %v)", svc, ip, ipKind)
	s.addServiceIPToKubeIPVSIntf(ip)
}
func (s *Backend) DeleteIP(svc *localnetv1.Service, ip string, ipKind serviceevents.IPKind) {
	klog.V(2).Infof("DeleteIP (svc: %v, svc-ip: %v, type: %v)", svc, ip, ipKind)
	s.deleteServiceIPToKubeIPVSIntf(ip)
}

// Handle session affinity
var _ serviceevents.SessionAffinityListener = &Backend{}

func (s *Backend) EnableSessionAffinity(svc *localnetv1.Service, sessionAffinity serviceevents.SessionAffinity) {
	klog.V(2).Infof("EnableSessionAffinity (svc: %v, sessionAffinity: %v)", svc, sessionAffinity)
	s.enableSessionAffinityForServiceIPs(svc, sessionAffinity)
}

func (s *Backend) DisableSessionAffinity(svc *localnetv1.Service) {
	klog.V(2).Infof("DisableSessionAffinity (svc: %v,)", svc)
	s.disableSessionAffinityForServiceIPs(svc)
}

// Handle traffic policy
var _ serviceevents.TrafficPolicyListener = &Backend{}

func (s *Backend) EnableTrafficPolicy(svc *localnetv1.Service, policyKind serviceevents.TrafficPolicyKind) {
	klog.V(2).Infof("EnableTrafficPolicy (svc: %v, policyKind: %v)", svc, policyKind)
}

func (s *Backend) DisableTrafficPolicy(svc *localnetv1.Service, policyKind serviceevents.TrafficPolicyKind) {
	klog.V(2).Infof("DisableTrafficPolicy (svc: %v, policyKind: %v)", svc, policyKind)
}

// SetService ------------------------------------------------------
// Service
//
func (s *Backend) SetService(svc *localnetv1.Service) {}

func (s *Backend) DeleteService(namespace, name string) {}

func (s *Backend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	klog.V(2).Infof("SetEndpoint(%q, %q, %q, %v)", namespace, serviceName, key, endpoint)
	svcKey := namespace + "/" + serviceName
	//TODO Check whether IPVS handles headless service
	if _, ok := s.svcs[svcKey]; !ok {
		klog.Infof("service (%s) could be headless-service", serviceName)
		return
	}
	service := s.svcs[svcKey]
	s.svcEPMap[svcKey]++

	if service.Type == ClusterIPService {
		s.handleEndPointForClusterIP(svcKey, key, endpoint, AddEndPoint)
	}

	if service.Type == NodePortService {
		s.handleEndPointForNodePortService(svcKey, key, endpoint, AddEndPoint)
	}

	if service.Type == LoadBalancerService {
		s.handleEndPointForLBService(svcKey, key, endpoint, AddEndPoint)
	}
}

func (s *Backend) DeleteEndpoint(namespace, serviceName, key string) {
	klog.V(2).Infof("DeleteEndpoint(%q, %q, %q)", namespace, serviceName, key)
	svcKey := namespace + "/" + serviceName
	//TODO Check whether IPVS handles headless service
	if _, ok := s.svcs[svcKey]; !ok {
		klog.Infof("service (%s) could be headless-service", serviceName)
		return
	}
	service := s.svcs[svcKey]
	s.svcEPMap[svcKey]--
	if service.Type == ClusterIPService {
		s.handleEndPointForClusterIP(svcKey, key, nil, DeleteEndPoint)
	}

	if service.Type == NodePortService {
		s.handleEndPointForNodePortService(svcKey, key, nil, DeleteEndPoint)
	}

	if service.Type == LoadBalancerService {
		s.handleEndPointForLBService(svcKey, key, nil, DeleteEndPoint)
	}
}

func (s *Backend) Setup() {
	kernelHandler := util.NewLinuxKernelHandler()
	err := s.initializeKernelConfig(kernelHandler)
	if err != nil {
		klog.Info(err)
		return
	}

	ipvs.Init()

	s.createIPVSDummyInterface()

	// Generate the masquerade mark to use for SNAT rules.
	//TODO fetch masqueradeBit from config
	masqueradeBit := 14
	masqueradeValue := 1 << uint(masqueradeBit)
	masqueradeMark := fmt.Sprintf("%#08x", masqueradeValue)

	// Create a ipset utils.
	execer := exec.New()
	ipsetInterface := util.New(execer)
	// Create iptables handlers for both families, one is already created
	// Always ordered as IPv4, IPv6
	// TODO check primaryProtocol based on IP Family of k8s cluster. Time being using as dual-stack always.
	var ipt [2]util.IPTableInterface
	ipt[0] = util.NewIPTableInterface(execer, util.ProtocolIPv4)
	ipt[1] = util.NewIPTableInterface(execer, util.ProtocolIPv6)

	detectLocalMode, err := getDetectLocalMode(s.k8sProxyConfig)
	if err != nil {
		return
	}

	klog.V(2).Info("DetectLocalMode", "LocalMode", string(detectLocalMode))

	var localDetector [2]util.LocalTrafficDetector
	localDetector, err = getDualStackLocalDetectorTuple(detectLocalMode, s.k8sProxyConfig, ipt, nil)
	klog.V(2).Info("LocalDetector return value is...", localDetector)
	if err != nil {
		return
	}

	s.NewDualStackProxier(ipsetInterface, masqueradeMark, localDetector, ipt)
}

func (s *Backend) NewDualStackProxier(ipsetInterface util.Interface,
	masqueradeMark string,
	localDetector [2]util.LocalTrafficDetector,
	iptInterface [2]util.IPTableInterface) {

	for i, ipFamily := range []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol} {
		var nodeIPs []string

		for _, nodeIP := range s.nodeAddresses {
			if ipFamily == getIPFamily(nodeIP) {
				nodeIPs = append(nodeIPs, nodeIP)
			}
		}

		s.proxiers[ipFamily] = NewProxier(
			ipFamily,
			s.dummy,
			ipsetInterface,
			iptInterface[i],
			nodeIPs,
			s.schedulingMethod,
			masqueradeMark,
			s.masqueradeAll,
			localDetector[i],
			s.weight,
		)

		s.proxiers[ipFamily].initializeIPSets()
	}
}
func (s *Backend) createIPVSDummyInterface() {
	// populate dummyIPs
	const dummyName = "kube-ipvs0"

	dummy, err := netlink.LinkByName(dummyName)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); !ok {
			klog.Fatal("failed to get dummy interface: ", err)
		}

		// not found => create the dummy
		dummy = &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{Name: dummyName},
		}

		klog.Info("creating dummy interface ", dummyName)
		if err = netlink.LinkAdd(dummy); err != nil {
			klog.Fatal("failed to create dummy interface: ", err)
		}

		dummy, err = netlink.LinkByName(dummyName)
		if err != nil {
			klog.Fatal("failed to get link after create: ", err)
		}
	}

	if dummy.Attrs().Flags&net.FlagUp == 0 {
		klog.Info("setting dummy interface ", dummyName, " up")
		if err = netlink.LinkSetUp(dummy); err != nil {
			klog.Fatal("failed to set dummy interface up: ", err)
		}
	}

	s.dummy = dummy

	dummyIface, err := net.InterfaceByName(dummyName)
	if err != nil {
		klog.Fatal("failed to get dummy interface: ", err)
	}

	addrs, err := dummyIface.Addrs()
	if err != nil {
		klog.Fatal("failed to list dummy interface IPs: ", err)
	}

	for _, ip := range addrs {
		cidr := ip.String()
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			klog.Fatalf("failed to parse ip/net %q: %v", ip, err)
		}
		if ip.IsLinkLocalUnicast() {
			continue
		}
	}
}

// WaitRequest see localsink.Sink#WaitRequest
func (s *Backend) WaitRequest() (nodeName string, err error) {
	name, _ := os.Hostname()
	return name, nil
}

func (s *Backend) Reset() { /* noop, we're wrapped in filterreset */ }

func (s *Backend) Sync() {
	if log := klog.V(1); log {
		klog.Info("Sync()")

		start := time.Now()
		defer klog.Info("sync took ", time.Now().Sub(start))
	}

	for _, proxier := range s.proxiers {
		proxier.sync()
	}
}

func (s *Backend) addServiceIPToKubeIPVSIntf(serviceIP string) {
	ipFamily := getIPFamily(serviceIP)

	ip := asDummyIPs(serviceIP, ipFamily)

	_, ipNet, err := net.ParseCIDR(ip)
	if err != nil {
		klog.Fatalf("failed to parse ip/net %q: %v", ip, err)
	}
	klog.V(2).Info("adding dummy IP ", ip)
	if err = netlink.AddrAdd(s.dummy, &netlink.Addr{IPNet: ipNet}); err != nil {
		klog.Error("failed to add dummy IP ", ip, ": ", err)
	}
}

func (s *Backend) deleteServiceIPToKubeIPVSIntf(serviceIP string) {
	ipFamily := getIPFamily(serviceIP)

	ip := asDummyIPs(serviceIP, ipFamily)

	_, ipNet, err := net.ParseCIDR(ip)
	if err != nil {
		klog.Fatalf("failed to parse ip/net %q: %v", ip, err)
	}
	klog.V(2).Info("deleting dummy IP ", ip)
	if err = netlink.AddrDel(s.dummy, &netlink.Addr{IPNet: ipNet}); err != nil {
		klog.Error("failed to delete dummy IP ", ip, ": ", err)
	}
}

func (s *Backend) initializeKernelConfig(kernelHandler util.KernelHandler) error {
	// Proxy needs br_netfilter and bridge-nf-call-iptables=1 when containers
	// are connected to a Linux bridge (but not SDN bridges).  Until most
	// plugins handle this, log when config is missing
	sysctl := util.NewSysInterface()
	if val, err := sysctl.GetSysctl(sysctlBridgeCallIPTables); err == nil && val != 1 {
		klog.Info("Missing br-netfilter module or unset sysctl br-nf-call-iptables, proxy may not work as intended")
	}

	// Set the conntrack sysctl we need for
	if err := util.EnsureSysctl(sysctl, sysctlVSConnTrack, 1); err != nil {
		return err
	}

	kernelVersionStr, err := kernelHandler.GetKernelVersion()
	if err != nil {
		return fmt.Errorf("error determining kernel version to find required kernel modules for ipvs support: %v", err)
	}
	kernelVersion, err := version.ParseGeneric(kernelVersionStr)
	if err != nil {
		return fmt.Errorf("error parsing kernel version %q: %v", kernelVersionStr, err)
	}
	if kernelVersion.LessThan(version.MustParseGeneric(connReuseMinSupportedKernelVersion)) {
		klog.Error(nil, "Can't set sysctl, kernel version doesn't satisfy minimum version requirements", "sysctl", sysctlConnReuse, "minimumKernelVersion", connReuseMinSupportedKernelVersion)
	} else if kernelVersion.AtLeast(version.MustParseGeneric(connReuseFixedKernelVersion)) {
		// https://github.com/kubernetes/kubernetes/issues/93297
		klog.V(2).Info("Left as-is", "sysctl", sysctlConnReuse)
	} else {
		// Set the connection reuse mode
		if err := util.EnsureSysctl(sysctl, sysctlConnReuse, 0); err != nil {
			return err
		}
	}

	// Set the expire_nodest_conn sysctl we need for
	if err := util.EnsureSysctl(sysctl, sysctlExpireNoDestConn, 1); err != nil {
		return err
	}

	// Set the expire_quiescent_template sysctl we need for
	if err := util.EnsureSysctl(sysctl, sysctlExpireQuiescentTemplate, 1); err != nil {
		return err
	}

	// Set the ip_forward sysctl we need for
	if err := util.EnsureSysctl(sysctl, sysctlForward, 1); err != nil {
		return err
	}

	//if strictARP {
	//	// Set the arp_ignore sysctl we need for
	//	if err := utilproxy.EnsureSysctl(sysctl, sysctlArpIgnore, 1); err != nil {
	//		return err
	//	}
	//
	//	// Set the arp_announce sysctl we need for
	//	if err := utilproxy.EnsureSysctl(sysctl, sysctlArpAnnounce, 2); err != nil {
	//		return err
	//	}
	//}
	return nil
}

func getDualStackLocalDetectorTuple(mode config.LocalMode, cfg *config.KpngConfiguration, ipt [2]util.IPTableInterface, nodeInfo *v1.Node) ([2]util.LocalTrafficDetector, error) {
	var err error
	localDetectors := [2]util.LocalTrafficDetector{util.NewNoOpLocalDetector(), util.NewNoOpLocalDetector()}
	switch mode {
	case config.LocalModeClusterCIDR:
		if len(strings.TrimSpace(cfg.ClusterCIDR)) == 0 {
			klog.Info("Detect-local-mode set to ClusterCIDR, but no cluster CIDR defined")
			break
		}

		clusterCIDRs := cidrTuple(cfg.ClusterCIDR)

		if len(strings.TrimSpace(clusterCIDRs[0])) == 0 {
			klog.Info("Detect-local-mode set to ClusterCIDR, but no IPv4 cluster CIDR defined, defaulting to no-op detect-local for IPv4")
		} else {
			localDetectors[0], err = util.NewDetectLocalByCIDR(clusterCIDRs[0], ipt[0])
			if err != nil { // don't loose the original error
				return localDetectors, err
			}
		}

		if len(strings.TrimSpace(clusterCIDRs[1])) == 0 {
			klog.Info("Detect-local-mode set to ClusterCIDR, but no IPv6 cluster CIDR defined, , defaulting to no-op detect-local for IPv6")
		} else {
			localDetectors[1], err = util.NewDetectLocalByCIDR(clusterCIDRs[1], ipt[1])
		}
		return localDetectors, err
	case config.LocalModeNodeCIDR:
		if nodeInfo == nil || len(strings.TrimSpace(nodeInfo.Spec.PodCIDR)) == 0 {
			klog.Info("No node info available to configure detect-local-mode NodeCIDR")
			break
		}
		// localDetectors, like ipt, need to be of the order [IPv4, IPv6], but PodCIDRs is setup so that PodCIDRs[0] == PodCIDR.
		// so have to handle the case where PodCIDR can be IPv6 and set that to localDetectors[1]
		if netutils.IsIPv6CIDRString(nodeInfo.Spec.PodCIDR) {
			localDetectors[1], err = util.NewDetectLocalByCIDR(nodeInfo.Spec.PodCIDR, ipt[1])
			if err != nil {
				return localDetectors, err
			}
			if len(nodeInfo.Spec.PodCIDRs) > 1 {
				localDetectors[0], err = util.NewDetectLocalByCIDR(nodeInfo.Spec.PodCIDRs[1], ipt[0])
			}
		} else {
			localDetectors[0], err = util.NewDetectLocalByCIDR(nodeInfo.Spec.PodCIDR, ipt[0])
			if err != nil {
				return localDetectors, err
			}
			if len(nodeInfo.Spec.PodCIDRs) > 1 {
				localDetectors[1], err = util.NewDetectLocalByCIDR(nodeInfo.Spec.PodCIDRs[1], ipt[1])
			}
		}
		return localDetectors, err
	default:
		klog.Info("Unknown detect-local-mode", "detect-local-mode", mode)
	}
	klog.Info("Defaulting to no-op detect-local", "detect-local-mode", string(mode))
	return localDetectors, nil
}

func getDetectLocalMode(cfg *config.KpngConfiguration) (config.LocalMode, error) {
	mode := cfg.DetectLocalMode
	klog.Info("local mode is ......", cfg.DetectLocalMode)
	switch mode {
	case config.LocalModeClusterCIDR, config.LocalModeNodeCIDR:
		return mode, nil
	default:
		if strings.TrimSpace(mode.String()) != "" {
			return mode, fmt.Errorf("unknown detect-local-mode: %v", mode)
		}
		klog.V(4).Info("Defaulting detect-local-mode", "LocalModeClusterCIDR", string(config.LocalModeClusterCIDR))
		return config.LocalModeClusterCIDR, nil
	}
}

// cidrTuple takes a comma separated list of CIDRs and return a tuple (ipv4cidr,ipv6cidr)
// The returned tuple is guaranteed to have the order (ipv4,ipv6) and if no cidr from a family is found an
// empty string "" is inserted.
func cidrTuple(cidrList string) [2]string {
	cidrs := [2]string{"", ""}
	foundIPv4 := false
	foundIPv6 := false

	for _, cidr := range strings.Split(cidrList, ",") {
		if netutils.IsIPv6CIDRString(cidr) && !foundIPv6 {
			cidrs[1] = cidr
			foundIPv6 = true
		} else if !foundIPv4 {
			cidrs[0] = cidr
			foundIPv4 = true
		}
		if foundIPv6 && foundIPv4 {
			break
		}
	}

	return cidrs
}
