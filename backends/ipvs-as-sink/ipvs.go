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
	"net"
	"strings"
	"time"

	"github.com/google/seesaw/ipvs"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"
	"k8s.io/utils/exec"

	localnetv12 "sigs.k8s.io/kpng/api/localnetv1"
	ipvs2 "sigs.k8s.io/kpng/backends/ipvs/util"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
	diffstore2 "sigs.k8s.io/kpng/server/pkg/diffstore"
)

type Backend struct {
	localsink.Config

	dryRun           bool
	nodeAddresses    []string
	schedulingMethod string
	weight           int32

	dummy netlink.Link

	svcs map[string]*localnetv12.Service

	dummyIPsRefCounts map[string]int

	// <namespace>/<service-name>/<ip>/<protocol>:<port> -> ipvsLB
	lbs *diffstore2.DiffStore

	// <namespace>/<service-name>/<endpoint key>/<ip> -> <ip>
	endpoints *diffstore2.DiffStore

	// <namespace>/<service-name>/<ip>/<protocol>:<port>/<ip> -> ipvsSvcDst
	dests *diffstore2.DiffStore

	ipsetList map[string]*IPSet
}

var _ decoder.Interface = &Backend{}

func New() *Backend {
	return &Backend{
		svcs: map[string]*localnetv12.Service{},

		dummyIPsRefCounts: map[string]int{},

		lbs:       diffstore2.New(),
		endpoints: diffstore2.New(),
		dests:     diffstore2.New(),
	}
}

func (s *Backend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(s))
}

func (s *Backend) Setup() {
	ipvs.Init()

	s.createIPVSDummyInterface()

	s.initializeIPSets()
	// TODO: populate lbs and endpoints with some kind and "claim" mechanism, or just flush ipvs LBs?
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
		ip, _, _ := net.ParseCIDR(cidr)
		if ip.IsLinkLocalUnicast() {
			continue
		}

		s.dummyIPsRefCounts[cidr] = 0
	}
}

func (s *Backend) initializeIPSets() {
	var ipsetInterface ipvs2.Interface

	// Create a iptables utils.
	execer := exec.New()

	ipsetInterface = ipvs2.New(execer)

	// initialize ipsetList with all sets we needed
	s.ipsetList = make(map[string]*IPSet)
	for _, is := range ipsetInfo {
		ipv4Set := newIPv4Set(ipsetInterface, is.name, is.setType, is.comment)
		s.ipsetList[ipv4Set.Name] = ipv4Set

		ipv6Set := newIPv6Set(ipsetInterface, is.name, is.setType, is.comment)
		s.ipsetList[ipv6Set.Name] = ipv6Set
	}

	// make sure ip sets exists in the system.
	for _, set := range s.ipsetList {
		if err := ensureIPSet(set); err != nil {
			return
		}
	}
}

func (s *Backend) Reset() { /* noop, we're wrapped in filterreset */ }

func (s *Backend) Sync() {
	if log := klog.V(1); log {
		log.Info("Sync()")

		start := time.Now()
		defer log.Info("sync took ", time.Now().Sub(start))
	}

	// clear unused dummy IPs
	for ip, refCount := range s.dummyIPsRefCounts {
		if refCount == 0 {
			klog.V(2).Info("removing dummy IP ", ip)

			_, ipNet, err := net.ParseCIDR(ip)
			if err != nil {
				klog.Fatalf("failed to parse ip/net %q: %v", ip, err)
			}

			if err = netlink.AddrDel(s.dummy, &netlink.Addr{IPNet: ipNet}); err != nil {
				klog.Error("failed to del dummy IP ", ip, ": ", err)
			}

			delete(s.dummyIPsRefCounts, ip)
		}
	}

	// add service IP into IPVS
	for _, lbKV := range s.lbs.Updated() {
		lb := lbKV.Value.(ipvsLB)
		// add the service
		klog.V(2).Info("adding service ", string(lbKV.Key))

		ipvsSvc := lb.ToService()
		err := ipvs.AddService(ipvsSvc)

		if err != nil && !strings.HasSuffix(err.Error(), "object exists") {
			klog.Error("failed to add service in IPVS", string(lbKV.Key), ": ", err)
		}
	}

	// Add endpoint/real-server entries into IPVS
	for _, kv := range s.dests.Updated() {
		svcDst := kv.Value.(ipvsSvcDst)

		klog.V(2).Info("adding destination ", string(kv.Key))
		if err := ipvs.AddDestination(svcDst.Svc, svcDst.Dst); err != nil && !strings.HasSuffix(err.Error(), "object exists") {
			klog.Error("failed to add destination ", string(kv.Key), ": ", err)
		}
	}

	// Delete endpoint/real-server entries from IPVS
	for _, kv := range s.dests.Deleted() {
		svcDst := kv.Value.(ipvsSvcDst)

		klog.V(2).Info("deleting destination ", string(kv.Key))
		if err := ipvs.DeleteDestination(svcDst.Svc, svcDst.Dst); err != nil {
			klog.Error("failed to delete destination ", string(kv.Key), ": ", err)
		}
	}

	// remove service IP from IPVS
	for _, lbKV := range s.lbs.Deleted() {
		lb := lbKV.Value.(ipvsLB)

		klog.V(2).Info("deleting service ", string(lbKV.Key))
		err := ipvs.DeleteService(lb.ToService())

		if err != nil {
			klog.Error("failed to delete service from IPVS", string(lbKV.Key), ": ", err)
		}
	}

	// sync ipset entries
	for _, set := range s.ipsetList {
		set.syncIPSetEntries()
		set.resetEntries()
	}

	// signal diffstores we've finished
	s.lbs.Reset(diffstore2.ItemUnchanged)
	s.endpoints.Reset(diffstore2.ItemUnchanged)
	s.dests.Reset(diffstore2.ItemUnchanged)
}

func (s *Backend) SetService(svc *localnetv12.Service) {
	klog.V(1).Infof("SetService(%v)", svc)

	if svc.Type == ClusterIPService {
		s.handleClusterIPService(svc, AddService)
	}

	if svc.Type == NodePortService {
		s.handleNodePortService(svc, AddService)
	}
}

func (s *Backend) DeleteService(namespace, name string) {
	klog.V(1).Infof("DeleteService(%q, %q)", namespace, name)

	key := namespace + "/" + name
	svc := s.svcs[key]
	delete(s.svcs, key)

	if svc.Type == ClusterIPService {
		s.handleClusterIPService(svc, DeleteService)
	}

	if svc.Type == NodePortService {
		s.handleNodePortService(svc, DeleteService)
	}

	for _, ip := range asDummyIPs(svc.IPs.All()) {
		s.dummyIPsRefCounts[ip]--
	}

	// remove all LBs associated to the service
	s.lbs.DeleteByPrefix([]byte(key + "/"))
}

func (s *Backend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv12.Endpoint) {
	klog.Infof("SetEndpoint(%q, %q, %q, %v)", namespace, serviceName, key, endpoint)

	svcKey := namespace + "/" + serviceName
	service := s.svcs[svcKey]

	if service.Type == ClusterIPService {
		s.SetEndPointForClusterIPSvc(svcKey, key, endpoint)
	}

	if service.Type == NodePortService {
		s.SetEndPointForNodePortSvc(svcKey, key, endpoint)
	}
}

func (s *Backend) DeleteEndpoint(namespace, serviceName, key string) {
	klog.Infof("DeleteEndpoint(%q, %q, %q)", namespace, serviceName, key)

	svcKey := namespace + "/" + serviceName
	service := s.svcs[svcKey]

	if service.Type == ClusterIPService {
		s.DeleteEndPointForClusterIPSvc(svcKey, key)
	}

	if service.Type == NodePortService {
		s.DeleteEndPointForNodePortSvc(svcKey, key)
	}
}
