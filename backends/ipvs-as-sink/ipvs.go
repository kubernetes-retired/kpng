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
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/exec"
	"net"
	"os"
	iptablesutil "sigs.k8s.io/kpng/backends/iptables/util"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
	"sigs.k8s.io/kpng/client/serviceevents"
	"time"

	"github.com/google/seesaw/ipvs"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
)

func init() {
	backendcmd.Register("to-ipvs", func() backendcmd.Cmd { return New() })
}

type Backend struct {
	localsink.Config
	svcs map[string]*localnetv1.Service
	proxiers map[v1.IPFamily]*proxier
	svcEPMap map[string]int

	dryRun           bool
	nodeAddresses    []string
	schedulingMethod string
	weight           int32

	dummy netlink.Link

	masqueradeAll bool
	//masqueradeBit *int32
}

var _ decoder.Interface = &Backend{}

func New() *Backend {
	return &Backend{
		proxiers: make(map[v1.IPFamily]*proxier),
		svcs: map[string]*localnetv1.Service{},
		svcEPMap: map[string]int{},
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
	klog.V(2).Infof("AddIPPort (svc : %v, svc-ip: %v, port: %v)", svc, ip, port)
	if svc.Type == ClusterIPService {
		s.updateClusterIPService(svc, ip, port)
	}

	if svc.Type == NodePortService {
		s.updateNodePortService(svc, ip, port)
	}

	if svc.Type == LoadBalancerService {
		s.updateLbIPService(svc, ip, IPKind, port)
	}
}

func (s *Backend) DeleteIPPort(svc *localnetv1.Service, ip string, IPKind serviceevents.IPKind, port *localnetv1.PortMapping) {
	klog.V(2).Infof("DeleteIPPort (svc: %v, svc-ip: %v, port: %v)", svc, ip, port)
	if svc.Type == ClusterIPService {
		s.deleteClusterIPService(svc, ip, port)
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

// -------------------------------------------------------------------------
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
		s.handleEndPointForClusterIP(svcKey, key, service, endpoint, AddEndPoint)
	}

	if service.Type == NodePortService {
		s.handleEndPointForNodePortService(svcKey, key, service, endpoint, AddEndPoint)
	}

	if service.Type == LoadBalancerService {
		s.handleEndPointForLBService(svcKey, key, service, endpoint, AddEndPoint)
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
		s.handleEndPointForClusterIP(svcKey, key, service,nil, DeleteEndPoint)
	}

	if service.Type == NodePortService {
		s.handleEndPointForNodePortService(svcKey, key, service, nil, DeleteEndPoint)
	}

	if service.Type == LoadBalancerService {
		s.handleEndPointForLBService(svcKey, key, service, nil, DeleteEndPoint)
	}
}

func (s *Backend) Setup() {
	ipvs.Init()

	s.createIPVSDummyInterface()

	// Generate the masquerade mark to use for SNAT rules.
	//TODO fetch masqueradeBit from config
	masqueradeBit := 14
	masqueradeValue := 1 << uint(masqueradeBit)
	masqueradeMark := fmt.Sprintf("%#08x", masqueradeValue)

	// Create a ipset utils.
	execer := exec.New()
	ipsetInterface := ipsetutil.New(execer)

	for _, ipFamily := range []v1.IPFamily {v1.IPv4Protocol, v1.IPv6Protocol} {
		var nodeIPs []string

		for _, nodeIP := range s.nodeAddresses {
			if ipFamily == getIPFamily(nodeIP) {
				nodeIPs = append(nodeIPs, nodeIP)
			}
		}

		iptInterface := iptablesutil.New(execer, iptablesutil.Protocol(ipFamily))

		s.proxiers[ipFamily] = NewProxier(
			ipFamily,
			s.dummy,
			ipsetInterface,
			iptInterface,
			nodeIPs,
			s.schedulingMethod,
			masqueradeMark,
			s.masqueradeAll,
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
	name, _ := os.Hostname(); return name, nil
}

func (s *Backend) Reset() { /* noop, we're wrapped in filterreset */ }

func (s *Backend) Sync() {
	if log := klog.V(1); log {
		log.Info("Sync()")

		start := time.Now()
		defer log.Info("sync took ", time.Now().Sub(start))
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
