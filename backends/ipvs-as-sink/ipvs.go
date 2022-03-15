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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/klog"

	"time"

	"sigs.k8s.io/kpng/backends/ipvs-as-sink/exec"
	util "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
	"sigs.k8s.io/kpng/client/serviceevents"

	"github.com/google/seesaw/ipvs"
	"github.com/vishvananda/netlink"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
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

	k8sProxyConfig *KpngConfiguration
}

// KubeProxyConfiguration contains everything necessary to configure the
// Kubernetes proxy server.
type KpngConfiguration struct {
	metav1.TypeMeta

	// featureGates is a map of feature names to bools that enable or disable alpha/experimental features.
	FeatureGates map[string]bool

	// bindAddress is the IP address for the proxy server to serve on (set to 0.0.0.0
	// for all interfaces)
	BindAddress string
	// healthzBindAddress is the IP address and port for the health check server to serve on,
	// defaulting to 0.0.0.0:10256
	HealthzBindAddress string
	// metricsBindAddress is the IP address and port for the metrics server to serve on,
	// defaulting to 127.0.0.1:10249 (set to 0.0.0.0 for all interfaces)
	MetricsBindAddress string
	// BindAddressHardFail, if true, kube-proxy will treat failure to bind to a port as fatal and exit
	BindAddressHardFail bool
	// enableProfiling enables profiling via web interface on /debug/pprof handler.
	// Profiling handlers will be handled by metrics server.
	EnableProfiling bool
	// clusterCIDR is the CIDR range of the pods in the cluster. It is used to
	// bridge traffic coming from outside of the cluster. If not provided,
	// no off-cluster bridging will be performed.
	ClusterCIDR string
	// hostnameOverride, if non-empty, will be used as the identity instead of the actual hostname.
	HostnameOverride string
	// // clientConnection specifies the kubeconfig file and client connection settings for the proxy
	// // server to use when communicating with the apiserver.
	// ClientConnection componentbaseconfig.ClientConnectionConfiguration
	// ipvs contains ipvs-related configuration options.
	IPVS KpngIPVSConfiguration
	// oomScoreAdj is the oom-score-adj value for kube-proxy process. Values must be within
	// the range [-1000, 1000]
	OOMScoreAdj *int32
	// portRange is the range of host ports (beginPort-endPort, inclusive) that may be consumed
	// in order to proxy service traffic. If unspecified (0-0) then ports will be randomly chosen.
	PortRange string
	// udpIdleTimeout is how long an idle UDP connection will be kept open (e.g. '250ms', '2s').
	// Must be greater than 0. Only applicable for proxyMode=userspace.
	UDPIdleTimeout metav1.Duration
	// conntrack contains conntrack-related configuration options.
	Conntrack KpngConntrackConfiguration
	// configSyncPeriod is how often configuration from the apiserver is refreshed. Must be greater
	// than 0.
	ConfigSyncPeriod metav1.Duration
	// nodePortAddresses is the --nodeport-addresses value for kube-proxy process. Values must be valid
	// IP blocks. These values are as a parameter to select the interfaces where nodeport works.
	// In case someone would like to expose a service on localhost for local visit and some other interfaces for
	// particular purpose, a list of IP blocks would do that.
	// If set it to "127.0.0.0/8", kube-proxy will only select the loopback interface for NodePort.
	// If set it to a non-zero IP block, kube-proxy will filter that down to just the IPs that applied to the node.
	// An empty string slice is meant to select all network interfaces.
	NodePortAddresses []string
	// ShowHiddenMetricsForVersion is the version for which you want to show hidden metrics.
	ShowHiddenMetricsForVersion string
	// DetectLocalMode determines mode to use for detecting local traffic, defaults to LocalModeClusterCIDR
	DetectLocalMode LocalMode
}

// KubeProxyConntrackConfiguration contains conntrack settings for
// the Kubernetes proxy server.
type KpngConntrackConfiguration struct {
	// maxPerCore is the maximum number of NAT connections to track
	// per CPU core (0 to leave the limit as-is and ignore min).
	MaxPerCore *int32
	// min is the minimum value of connect-tracking records to allocate,
	// regardless of maxPerCore (set maxPerCore=0 to leave the limit as-is).
	Min *int32
	// tcpEstablishedTimeout is how long an idle TCP connection will be kept open
	// (e.g. '2s').  Must be greater than 0 to set.
	TCPEstablishedTimeout *metav1.Duration
	// tcpCloseWaitTimeout is how long an idle conntrack entry
	// in CLOSE_WAIT state will remain in the conntrack
	// table. (e.g. '60s'). Must be greater than 0 to set.
	TCPCloseWaitTimeout *metav1.Duration
}

// KubeProxyIPVSConfiguration contains ipvs-related configuration
// details for the Kubernetes proxy server.
type KpngIPVSConfiguration struct {
	// syncPeriod is the period that ipvs rules are refreshed (e.g. '5s', '1m',
	// '2h22m').  Must be greater than 0.
	SyncPeriod metav1.Duration
	// minSyncPeriod is the minimum period that ipvs rules are refreshed (e.g. '5s', '1m',
	// '2h22m').
	MinSyncPeriod metav1.Duration
	// ipvs scheduler
	Scheduler string
	// excludeCIDRs is a list of CIDR's which the ipvs proxier should not touch
	// when cleaning up ipvs services.
	ExcludeCIDRs []string
	// strict ARP configure arp_ignore and arp_announce to avoid answering ARP queries
	// from kube-ipvs0 interface
	StrictARP bool
	// tcpTimeout is the timeout value used for idle IPVS TCP sessions.
	// The default value is 0, which preserves the current timeout value on the system.
	TCPTimeout metav1.Duration
	// tcpFinTimeout is the timeout value used for IPVS TCP sessions after receiving a FIN.
	// The default value is 0, which preserves the current timeout value on the system.
	TCPFinTimeout metav1.Duration
	// udpTimeout is the timeout value used for IPVS UDP packets.
	// The default value is 0, which preserves the current timeout value on the system.
	UDPTimeout metav1.Duration
}

// LocalMode represents modes to detect local traffic from the node
type LocalMode string

// Currently supported modes for LocalMode
const (
	LocalModeClusterCIDR LocalMode = "ClusterCIDR"
	LocalModeNodeCIDR    LocalMode = "NodeCIDR"
)

func (m *LocalMode) Set(s string) error {
	*m = LocalMode(s)
	return nil
}

func (m *LocalMode) String() string {
	if m != nil {
		return string(*m)
	}
	return ""
}

func (m *LocalMode) Type() string {
	return "LocalMode"
}

var _ decoder.Interface = &Backend{}

func New() *Backend {
	return &Backend{
		proxiers:       make(map[v1.IPFamily]*proxier),
		svcs:           map[string]*localnetv1.Service{},
		svcEPMap:       map[string]int{},
		k8sProxyConfig: new(KpngConfiguration),
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

	for _, ipFamily := range []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol} {
		var nodeIPs []string

		for _, nodeIP := range s.nodeAddresses {
			if ipFamily == getIPFamily(nodeIP) {
				nodeIPs = append(nodeIPs, nodeIP)
			}
		}

		iptInterface := util.NewIPTableInterface(execer, util.Protocol(ipFamily))

		var config KpngConfiguration
		var detectLocalMode LocalMode
		detectLocalMode, err = s.getDetectLocalMode(config)
		if err != nil {
			return
		}

		var nodeInfo *v1.Node
		// TODO Implement the NodeCIDR mode.
		// if detectLocalMode == proxyconfigapi.LocalModeNodeCIDR {
		// 	klog.Info("Watching for node, awaiting podCIDR allocation", "hostname", hostname)
		// 	nodeInfo, err = waitForPodCIDR(client, hostname)
		// 	if err != nil {
		// 		return nil, err
		// 	}
		// 	klog.Info("NodeInfo", "PodCIDR", nodeInfo.Spec.PodCIDR, "PodCIDRs", nodeInfo.Spec.PodCIDRs)
		// }

		klog.V(2).Info("DetectLocalMode", "LocalMode", string(detectLocalMode))

		var localDetector util.LocalTrafficDetector
		localDetector, err = s.getLocalDetector(detectLocalMode, config, iptInterface, nodeInfo)
		klog.V(2).Info("LocalDetector return value is...", localDetector)
		if err != nil {
			return
		}

		s.proxiers[ipFamily] = NewProxier(
			ipFamily,
			s.dummy,
			ipsetInterface,
			iptInterface,
			nodeIPs,
			s.schedulingMethod,
			masqueradeMark,
			s.masqueradeAll,
			localDetector,
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

func (s *Backend) getLocalDetector(mode LocalMode, config KpngConfiguration, ipt util.IPTableInterface, nodeInfo *v1.Node) (util.LocalTrafficDetector, error) {
	switch mode {
	case LocalModeClusterCIDR:
		if len(strings.TrimSpace(config.ClusterCIDR)) == 0 {
			klog.Info("Detect-local-mode set to ClusterCIDR, but no cluster CIDR defined")
			klog.Info("clusterCIDR value is....", config.ClusterCIDR)
			break
		}
		return util.NewDetectLocalByCIDR(config.ClusterCIDR, ipt)
	case LocalModeNodeCIDR:
		if len(strings.TrimSpace(nodeInfo.Spec.PodCIDR)) == 0 {
			klog.Info("Detect-local-mode set to NodeCIDR, but no PodCIDR defined at node")
			break
		}
		return util.NewDetectLocalByCIDR(nodeInfo.Spec.PodCIDR, ipt)
	}
	klog.V(0).Info("Defaulting to no-op detect-local", "detect-local-mode", string(mode))
	return util.NewNoOpLocalDetector(), nil
}

func (s *Backend) getDetectLocalMode(config KpngConfiguration) (LocalMode, error) {
	mode := config.DetectLocalMode
	klog.Info("local mode is ......", config.DetectLocalMode)
	switch mode {
	case LocalModeClusterCIDR, LocalModeNodeCIDR:
		return mode, nil
	default:
		if strings.TrimSpace(mode.String()) != "" {
			return mode, fmt.Errorf("unknown detect-local-mode: %v", mode)
		}
		klog.V(4).Info("Defaulting detect-local-mode", "LocalModeClusterCIDR", string(LocalModeClusterCIDR))
		return LocalModeClusterCIDR, nil
	}
}
