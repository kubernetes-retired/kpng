package ipvssink

import (
	"bytes"
	"net"
	"strings"
	"time"

	"github.com/google/seesaw/ipvs"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"

	"sigs.k8s.io/kpng/localsink"
	"sigs.k8s.io/kpng/localsink/decoder"
	"sigs.k8s.io/kpng/localsink/filterreset"
	"sigs.k8s.io/kpng/pkg/api/localnetv1"
	"sigs.k8s.io/kpng/pkg/diffstore"
)

type Backend struct {
	localsink.Config

	dryRun           bool
	nodeAddresses    []string
	schedulingMethod string
	weight           int32

	dummy netlink.Link

	svcs map[string]*localnetv1.Service

	dummyIPsRefCounts map[string]int

	// <namespace>/<service-name>/<ip>/<protocol>:<port> -> ipvsLB
	lbs *diffstore.DiffStore

	// <namespace>/<service-name>/<endpoint key>/<ip> -> <ip>
	endpoints *diffstore.DiffStore

	// <namespace>/<service-name>/<ip>/<protocol>:<port>/<ip> -> ipvsSvcDst
	dests *diffstore.DiffStore
}

var _ decoder.Interface = &Backend{}

func New() *Backend {
	return &Backend{
		svcs: map[string]*localnetv1.Service{},

		dummyIPsRefCounts: map[string]int{},

		lbs:       diffstore.New(),
		endpoints: diffstore.New(),
		dests:     diffstore.New(),
	}
}

func (s *Backend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(s))
}

func (s *Backend) Setup() {
	ipvs.Init()

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

	// TODO: populate lbs and endpoints with some kind and "claim" mechanism, or just flush ipvs LBs?
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

	// add service LBs
	for _, lbKV := range s.lbs.Updated() {
		lb := lbKV.Value.(ipvsLB)
		// add the service
		klog.V(2).Info("adding service ", string(lbKV.Key))

		ipvsSvc := lb.ToService()
		err := ipvs.AddService(ipvsSvc)

		if err != nil && !strings.HasSuffix(err.Error(), "object exists") {
			klog.Error("failed to add service ", string(lbKV.Key), ": ", err)
		}

		// When existing service gets updated with new port/protocol, endpoints
		// behind it also needs to be updated into tree, so that they are handled below.
		for _, epKV := range s.endpoints.GetByPrefix([]byte(lb.ServiceKey + "/")) {
			epIP := epKV.Value.(string)
			s.dests.Set([]byte(string(lbKV.Key)+"/"+epIP), 0, ipvsSvcDst{
				Svc: lb.ToService(),
				Dst: ipvsDestination(epIP, lb.Port, s.weight),
			})
		}
	}

	// add/remove destinations
	for _, kv := range s.dests.Deleted() {
		svcDst := kv.Value.(ipvsSvcDst)

		klog.V(2).Info("deleting destination ", string(kv.Key))
		if err := ipvs.DeleteDestination(svcDst.Svc, svcDst.Dst); err != nil {
			klog.Error("failed to delete destination ", string(kv.Key), ": ", err)
		}
	}

	for _, kv := range s.dests.Updated() {
		svcDst := kv.Value.(ipvsSvcDst)

		klog.V(2).Info("adding destination ", string(kv.Key))
		if err := ipvs.AddDestination(svcDst.Svc, svcDst.Dst); err != nil && !strings.HasSuffix(err.Error(), "object exists") {
			klog.Error("failed to add destination ", string(kv.Key), ": ", err)
		}
	}

	// remove service LBs
	for _, lbKV := range s.lbs.Deleted() {
		lb := lbKV.Value.(ipvsLB)

		klog.V(2).Info("deleting service ", string(lbKV.Key))
		err := ipvs.DeleteService(lb.ToService())

		if err != nil {
			klog.Error("failed to delete service", string(lbKV.Key), ": ", err)
		}

		// When existing service gets updated with deletion of port/protocol,
		// endpoint behind it needs to be removed from tree.
		for _, epKV := range s.endpoints.GetByPrefix([]byte(lb.ServiceKey + "/")) {
			epIP := epKV.Value.(string)
			s.dests.Delete([]byte(string(lbKV.Key) + "/" + epIP))
		}
	}

	// signal diffstores we've finished
	s.lbs.Reset(diffstore.ItemUnchanged)
	s.endpoints.Reset(diffstore.ItemUnchanged)
	s.dests.Reset(diffstore.ItemUnchanged)
}

func (s *Backend) SetService(svc *localnetv1.Service) {
	klog.V(1).Infof("SetService(%v)", svc)

	if svc.Type == NodePortService || svc.Type == LoadBalancerService {
		s.handleNodePortSvc(svc)
	}

	if svc.Type == ClusterIPService {
		s.handleClusterIPSvc(svc)
	}
}

func (s *Backend) DeleteService(namespace, name string) {
	klog.V(1).Infof("DeleteService(%q, %q)", namespace, name)

	key := namespace + "/" + name
	svc := s.svcs[key]
	delete(s.svcs, key)

	for _, ip := range asDummyIPs(svc.IPs.All()) {
		s.dummyIPsRefCounts[ip]--
	}

	// remove all LBs associated to the service
	s.lbs.DeleteByPrefix([]byte(key + "/"))
}

func (s *Backend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	klog.Infof("SetEndpoint(%q, %q, %q, %v)", namespace, serviceName, key, endpoint)

	svcKey := namespace + "/" + serviceName
	prefix := svcKey + "/" + key + "/"

	for _, ips := range [][]string{endpoint.IPs.V4, endpoint.IPs.V6} {
		if len(ips) == 0 {
			continue
		}

		ip := ips[0]
		s.endpoints.Set([]byte(prefix+ip), 0, ip)

		// add a destination for every LB of this service
		for _, lbKV := range s.lbs.GetByPrefix([]byte(svcKey + "/")) {
			lb := lbKV.Value.(ipvsLB)
			s.dests.Set([]byte(string(lbKV.Key)+"/"+ip), 0, ipvsSvcDst{
				Svc: lb.ToService(),
				Dst: ipvsDestination(ip, lb.Port, s.weight),
			})
		}
	}

}

func (s *Backend) DeleteEndpoint(namespace, serviceName, key string) {
	klog.Infof("DeleteEndpoint(%q, %q, %q)", namespace, serviceName, key)

	svcPrefix := []byte(namespace + "/" + serviceName + "/")
	prefix := []byte(string(svcPrefix) + key + "/")

	for _, kv := range s.endpoints.GetByPrefix(prefix) {
		// remove this endpoint from the destinations if the service
		ip := kv.Value.(string)
		suffix := []byte("/" + ip)

		for _, destKV := range s.dests.GetByPrefix(svcPrefix) {
			if bytes.HasSuffix(destKV.Key, suffix) {
				s.dests.Delete(destKV.Key)
			}
		}
	}

	// remove this endpoint from the endpoints
	s.endpoints.DeleteByPrefix(prefix)
}

func (s *Backend) handleClusterIPSvc(svc *localnetv1.Service) {
	key := svc.Namespace + "/" + svc.Name

	isNewService := true
	if _, ok := s.svcs[key]; ok {
		isNewService = false
	}

	if isNewService {
		s.handleNewClusterIPSvc(key, svc)
	} else {
		s.handleUpdatedClusterIPSvc(key, svc)
	}
}

func (s *Backend) handleNewClusterIPSvc(key string, svc *localnetv1.Service) {
	// update the svc
	prevSvc := s.svcs[key]
	s.svcs[key] = svc

	s.addServiceIPToKubeIPVSIntf(prevSvc, svc)

	s.storeLBSvc(svc.Ports, svc.IPs.All().All(), key, ClusterIPService)
}

func (s *Backend) handleUpdatedClusterIPSvc(key string, svc *localnetv1.Service) {
	// update the svc
	prevSvc := s.svcs[key]
	s.svcs[key] = svc

	s.addServiceIPToKubeIPVSIntf(prevSvc, svc)

	addedPorts, removedPorts := diffInPortMapping(prevSvc, svc)

	s.storeLBSvc(addedPorts, svc.IPs.All().All(), key, ClusterIPService)

	s.deleteLBSvc(removedPorts, svc.IPs.All().All(), key)
}

func (s *Backend) handleNodePortSvc(svc *localnetv1.Service) {
	key := svc.Namespace + "/" + svc.Name

	isNewService := true
	if _, ok := s.svcs[key]; ok {
		isNewService = false
	}

	if isNewService {
		s.handleNewNodePortSvc(key, svc)
	} else {
		s.handleUpdatedNodePortSvc(key, svc)
	}

}

func (s *Backend) handleNewNodePortSvc(key string, svc *localnetv1.Service) {
	// update the svc
	prevSvc := s.svcs[key]
	s.svcs[key] = svc

	s.addServiceIPToKubeIPVSIntf(prevSvc, svc)

	//Node Addresses need to be added as NodePortService
	//so that in sync(), nodePort is attached to nodeIPs.
	s.storeLBSvc(svc.Ports, s.nodeAddresses, key, NodePortService)

	//NodePort svc clusterIPs need to be added as ClusterIPService
	//so that in sync(), port is attached to clusterIP.
	s.storeLBSvc(svc.Ports, svc.IPs.All().All(), key, ClusterIPService)
}

func (s *Backend) handleUpdatedNodePortSvc(key string, svc *localnetv1.Service) {
	// update the svc
	prevSvc := s.svcs[key]
	s.svcs[key] = svc

	s.addServiceIPToKubeIPVSIntf(prevSvc, svc)

	addedPorts, removedPorts := diffInPortMapping(prevSvc, svc)

	//Node Addresses need to be added as NodePortService
	//so that in sync(), nodePort is attached to nodeIPs.
	s.storeLBSvc(addedPorts, s.nodeAddresses, key, NodePortService)

	//NodePort svc clusterIPs need to be added as ClusterIPService
	//so that in sync(), port is attached to clusterIP.
	s.storeLBSvc(addedPorts, svc.IPs.All().All(), key, ClusterIPService)

	s.deleteLBSvc(removedPorts, s.nodeAddresses, key)

	s.deleteLBSvc(removedPorts, svc.IPs.All().All(), key)
}

func (s *Backend) addServiceIPToKubeIPVSIntf(prevSvc, curr *localnetv1.Service) {
	// sync dummy IPs
	var prevIPs *localnetv1.IPSet
	if prevSvc == nil {
		prevIPs = localnetv1.NewIPSet()
	} else {
		prevIPs = prevSvc.IPs.All()
	}

	currentIPs := curr.IPs.All()

	added, removed := prevIPs.Diff(currentIPs)

	for _, ip := range asDummyIPs(added) {
		if _, ok := s.dummyIPsRefCounts[ip]; !ok {
			// IP is not referenced so we must add it
			klog.V(2).Info("adding dummy IP ", ip)

			_, ipNet, err := net.ParseCIDR(ip)
			if err != nil {
				klog.Fatalf("failed to parse ip/net %q: %v", ip, err)
			}

			if err = netlink.AddrAdd(s.dummy, &netlink.Addr{IPNet: ipNet}); err != nil {
				klog.Error("failed to add dummy IP ", ip, ": ", err)
			}
		}

		s.dummyIPsRefCounts[ip]++
	}

	for _, ip := range asDummyIPs(removed) {
		s.dummyIPsRefCounts[ip]--
	}
}

func (s *Backend) storeLBSvc(portList []*localnetv1.PortMapping, addrList []string, key, svcType string) {
	for _, ip := range addrList {
		prefix := key + "/" + ip + "/"
		for _, port := range portList {
			lbKey := prefix + epPortSuffix(port)
			s.lbs.Set([]byte(lbKey), 0, ipvsLB{IP: ip, ServiceKey: key, Port: port, SchedulingMethod: s.schedulingMethod, ServiceType: svcType})
		}
	}
}

func (s *Backend) deleteLBSvc(portList []*localnetv1.PortMapping, addrList []string, key string) {
	for _, ip := range addrList {
		prefix := key + "/" + ip + "/"
		for _, port := range portList {
			lbKey := prefix + epPortSuffix(port)
			s.lbs.Delete([]byte(lbKey))
		}
	}
}
