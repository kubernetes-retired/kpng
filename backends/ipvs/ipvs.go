package ipvs

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/cespare/xxhash"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog"

	utilnet "k8s.io/utils/net"
	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/pkg/api/localnetv1"
)

var (
	clusterIPs map[string]interface{}
)

func Callback(ch <-chan *client.ServiceEndpoints) {
	clusterIPs = make(map[string]interface{})

	for serviceEndpoints := range ch {
		svc := serviceEndpoints.Service
		endpoints := serviceEndpoints.Endpoints
		// If this is a ClusterIP non headless
		if (svc.Type == "ClusterIP" && !svc.IPs.Headless) || svc.Type == "NodePort" || svc.Type == "LoadBalancer" {
			// TODO: Verify if the IP Address exists on the dummy interface, otherwise it need to be added
			for _, ip := range svc.IPs.ClusterIPs.All() {
				clusterIPs[ip] = nil

				// Future art: build some tree as Mikael did in nft, and verify if and where are the differences
				// to remove unused ClusterIP Addresses
				var nodePort bool
				if svc.Type == "NodePort" || svc.Type == "LoadBalancer" {
					nodePort = true
				}
				err := buildClusterIP(svc, ip, endpoints, nodePort)
				if err != nil {
					klog.Warningf("problem creating the service: %s", err)
				}
			}
		}
	}

	// check if we have changes to apply
	if !ipvsTable4.Changed() && !ipvsTable6.Changed() {
		klog.V(1).Info("no changes to apply")
		return
	}
	renderIPVSTables()

	createDummyInterfaceOnNode()

	ipvsTable4.Reset()
	ipvsTable6.Reset()
}

func buildClusterIP(svc *localnetv1.Service, svcIP string, eps []*localnetv1.Endpoint, nodePort bool) error {
	for _, port := range svc.Ports {
		var proto string
		switch port.Protocol {
		case localnetv1.Protocol_TCP:
			proto = "-t"
		case localnetv1.Protocol_SCTP:
			proto = "--sctp-service "
		case localnetv1.Protocol_UDP:
			proto = "-u"
		default:
			return fmt.Errorf("service %s/%s uses an unknown protocol", svc.Namespace, svc.Name)
		}
		tgtPort := port.GetTargetPort()
		if tgtPort < 1 {
			tgtPort = port.GetPort()
		}

		storeVirtualService(svc, port.Port, proto, svcIP)

		storeRealServer(svc, svcIP, proto, port.Port, port.TargetPort, eps)

		if nodePort {
			for _, address := range *NodeAddress {
				storeVirtualService(svc, port.NodePort, proto, address)

				storeRealServer(svc, address, proto, port.NodePort, port.TargetPort, eps)
			}
		}
	}
	return nil
}

func storeVirtualService(svc *localnetv1.Service, port int32, proto, svcIP string) {
	buffer := ipvsTable4
	if utilnet.IsIPv6String(svcIP) {
		buffer = ipvsTable6
	}
	virtualService := buffer.Get(virtualService, "virtual-service", getVirtualServiceName(svc, proto, svcIP, port))
	virtualService.WriteVirtualServiceInfo(proto, svcIP, *SchedulingMethod, port)
	klog.V(2).Infof("virtual-service IP:%s , protocol:%s, svcPort: %s", svcIP, proto, port)
}

func storeRealServer(svc *localnetv1.Service, svcIP, proto string, port, tgtPort int32, endpoints []*localnetv1.Endpoint) {
	for _, endpoint := range endpoints {
		ip := endpoint.GetIPs().V4[0]
		buffer := ipvsTable4
		if utilnet.IsIPv6String(svcIP) {
			ip = endpoint.GetIPs().V6[0]
			buffer = ipvsTable6
		}
		realServer := buffer.Get(realServer, "real-server", getRealServerName(svc, proto, svcIP, ip, tgtPort))
		realServer.WriteRealServerInfo(ip, tgtPort, proto, svcIP, port)
		klog.V(2).Infof("real-server IP:%s , protocol:%s, targetPort: %s", ip, proto, tgtPort)
	}
}

func getVirtualServiceName(svc *localnetv1.Service, proto, svcIP string, port int32) string {
	mapH := xxhash.Sum64String(svc.Name + "/" + svc.Namespace + "/" + proto + "/" + svcIP)
	virtualServiceName := fmt.Sprintf("vs_%d_%04x", port, mapH)
	return virtualServiceName
}

func getRealServerName(svc *localnetv1.Service, proto, svcIP, endpointIP string, port int32) string {
	mapH := xxhash.Sum64String(svc.Name + "/" + svc.Namespace + "/" + proto + "/" + svcIP + "/" + endpointIP)
	realServerName := fmt.Sprintf("rs_%d_%04x", port, mapH)
	return realServerName
}

func createDummyInterfaceOnNode() {
	dummyIface, err := net.InterfaceByName("kube-ipvs0")
	if err != nil {
		klog.Errorf("failed to get dummy interface: %s", err)
	}
	addrs, err := dummyIface.Addrs()
	if err != nil {
		klog.Errorf("failed to get dummy interface addresses: %s", err)
	}
	for _, v := range addrs {
		addr, _, err := net.ParseCIDR(v.String())
		if addr == nil {
			klog.Errorf("error parse ip address: %s", v.String())
			continue
		}
		if err != nil {
			klog.Errorf("error parse ip address: %s - %s", v.String(), err)
			continue
		}
		// SKIP IPv
		if addr.IsLinkLocalUnicast() {
			continue
		}
		if _, ok := clusterIPs[v.String()]; !ok {
			// Delete the interface address
			if err := netlink.AddrDel(
				&netlink.Dummy{
					LinkAttrs: netlink.LinkAttrs{Name: "kube-ipvs0"},
				},
				&netlink.Addr{IPNet: netlink.NewIPNet(addr)}); err != nil {
				if err != unix.ENXIO {
					klog.Errorf("error unbind address: %s from interface: %s, err: %v", addr, "kube-ipvs0", err)
				}
			}
		}
	}
	for clusteraddr, _ := range clusterIPs {
		addr := net.ParseIP(clusteraddr)
		if addr == nil {
			klog.Errorf("error parse ip address: %s", addr)
			continue
		}
		if err := netlink.AddrAdd(&netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{Name: "kube-ipvs0"},
		},
			&netlink.Addr{IPNet: netlink.NewIPNet(addr)}); err != nil {
			// "EEXIST" will be returned if the address is already bound to device
			if err == unix.EEXIST {
				continue
			}
			klog.Errorf("error bind address: %s to interface: %s, err: %v", clusteraddr, "kube-ipvs0", err)
		}

	}
}

func executeIPVSCmd(ipvsCfg string) {
	klog.V(2).Infof("Running restore for cmd: %s", ipvsCfg)
	var stderr bytes.Buffer
	ipvsRestore := exec.Command(*IPVSAdmPath, "--restore")
	ipvsRestore.Stdin = strings.NewReader(ipvsCfg)
	ipvsRestore.Stderr = &stderr

	err := ipvsRestore.Run()
	if err != nil {
		klog.Errorf("failed to execute ipvsadm restore: %s , %s", err, stderr.String())
	}
}

func renderIPVSTables() {
	for _, table := range []*ipvsTable{ipvsTable4, ipvsTable6} {
		handleVirtualServiceUpdates(table)

		handleRealServerUpdates(table)
	}
}
func handleVirtualServiceUpdates(table *ipvsTable) {
	// delete virtual-service
	for _, dvs := range table.DeletedVirtualService() {
		delVirtSvc := fmt.Sprintf("-D %s %s:%d\n", dvs.virtualService.protocol, dvs.virtualService.serviceIP, dvs.virtualService.servicePort)
		executeIPVSCmd(delVirtSvc)
	}

	//Get updated list
	list := table.ListOfVirtualService()
	filteredList := filterUpdatedInfo(virtualService, table, list)

	// create/update changed virtual-service entries
	if len(filteredList) != 0 {
		for _, vs := range filteredList {
			c := table.Get(virtualService, "", vs)
			virtSvc := fmt.Sprintf("-A %s %s:%d -s %s\n", c.virtualService.protocol, c.virtualService.serviceIP,
				c.virtualService.servicePort, c.virtualService.schedulingMethod)
			executeIPVSCmd(virtSvc)
		}
	}
}

func handleRealServerUpdates(table *ipvsTable) {
	// delete real-servers
	for _, drs := range table.DeletedRealServer() {
		delRealSrv := fmt.Sprintf("-d %s %s:%d -r %s:%d\n", drs.virtualService.protocol, drs.virtualService.serviceIP,
			drs.virtualService.servicePort, drs.realServer.endPointIP, drs.realServer.targetPort)
		executeIPVSCmd(delRealSrv)
	}

	//Get updated list
	list := table.ListOfRealServer()
	filteredList := filterUpdatedInfo(realServer, table, list)

	// create/update changed real-server entries
	if len(filteredList) != 0 {
		for _, rs := range filteredList {
			c := table.Get(realServer, "", rs)
			realSrv := fmt.Sprintf("-a %s %s:%d -r %s:%d -m -w %d\n", c.virtualService.protocol,
				c.virtualService.serviceIP, c.virtualService.servicePort, c.realServer.endPointIP,
				c.realServer.targetPort, *Weight)
			executeIPVSCmd(realSrv)
		}
	}
}

func filterUpdatedInfo(nodeType NodeType, table *ipvsTable, list []string) []string {
	filteredList := make([]string, 0, len(list))
	for _, item := range list {
		c := table.Get(nodeType, "", item)
		if !c.Changed() {
			continue
		}
		if !c.Created() {
		}
		filteredList = append(filteredList, item)
	}
	return filteredList
}
