package ipvs

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog"

	"sigs.k8s.io/kpng/client"
	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

const (
	// TODO: This is configurable :)
	DefaultAlgo   = "rr"
	DefaultWeight = 1
)

var (
	clusterIPs map[string]interface{}
)

func Callback(ch <-chan *client.ServiceEndpoints) {

	var ipvsCfg strings.Builder
	var err error
	clusterIPs = make(map[string]interface{})

	for serviceEndpoints := range ch {

		svc := serviceEndpoints.Service
		endpoints := serviceEndpoints.Endpoints

		//var svcRule strings.Builder
		// If this is a ClusterIP non headless
		if (svc.Type == "ClusterIP" && !svc.IPs.Headless) || svc.Type == "NodePort" || svc.Type == "LoadBalancer" {
			// TODO: Verify if the IP Address exists on the dummy interface, otherwise it need to be
			// added
			for _, ip := range svc.IPs.ClusterIPs.All() {
				clusterIPs[ip] = nil

				// Future art: build some tree as Mikael did in nft, and verify if and where are the differences
				// to remove unused ClusterIP Addresses
				var nodePort bool
				if svc.Type == "NodePort" || svc.Type == "LoadBalancer" {
					nodePort = true
				}
				cip, err := buildClusterIP(svc, ip, endpoints, nodePort)
				if err != nil {
					klog.Warningf("problem creating the service: %s", err)
				}
				ipvsCfg.WriteString(cip)
			}
		}
	}
	fmt.Printf("%s", ipvsCfg.String())

	if OnlyOutput != nil && !*OnlyOutput {
		fmt.Println("Running clear")
		ipvsClear := exec.Command(*IPVSAdmPath, "--clear")

		err = ipvsClear.Run()
		if err != nil {
			klog.Errorf("failed to clear ipvs table: %s", err)
		}

		fmt.Println("Running restore")
		ipvsRestore := exec.Command(*IPVSAdmPath, "--restore")
		ipvsRestore.Stdin = strings.NewReader(ipvsCfg.String())

		err = ipvsRestore.Run()
		if err != nil {
			klog.Errorf("failed to execute ipvsadm restore: %s", err)
		}
	}
	ipvsCfg.Reset()

	dummyIface, err := net.InterfaceByName("kube-ipvs0")
	if err != nil {
		klog.Errorf("failed to get dummy interface: %s", err)
	}
	addrs, err := dummyIface.Addrs()
	if err != nil {
		klog.Errorf("failed to get dummy interface addresses: %s", err)
	}

	// ipvs requires you to attach new service IPs to an interface

	// loop through addresses and clean them if not active anymore
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

	// add new addresses
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

func buildClusterIP(svc *localnetv1.Service, ip string, eps []*localnetv1.Endpoint, nodePort bool) (string, error) {
	var svcString strings.Builder
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
			return "", fmt.Errorf("service %s/%s uses an unknown protocol", svc.Namespace, svc.Name)
		}
		tgtPort := port.GetTargetPort()
		if tgtPort < 1 {
			tgtPort = port.GetPort()
		}
		ipPortAlgo := fmt.Sprintf("-A %s %s:%d -s %s\n", proto, ip, port.Port, DefaultAlgo)
		svcString.WriteString(ipPortAlgo)

		endpoints := buildEndponts(ip, proto, port.Port, port.TargetPort, eps)
		svcString.WriteString(endpoints)

		if nodePort {
			for _, address := range *NodeAddress {
				ipPortAlgo := fmt.Sprintf("-A %s %s:%d -s %s\n", proto, address, port.NodePort, DefaultAlgo)
				svcString.WriteString(ipPortAlgo)

				endpoints := buildEndponts(address, proto, port.NodePort, port.TargetPort, eps)
				svcString.WriteString(endpoints)

			}
		}
	}
	return svcString.String(), nil
}

func buildEndponts(VirtualIP, proto string, port, tgtPort int32, endpoints []*localnetv1.Endpoint) string {
	var strBuilder strings.Builder
	for _, endpoint := range endpoints {
		ip := endpoint.GetIPs().V4[0] //TODO:  This is what we call a Gambiarra :D Need to deal better with this thing.
		ep := fmt.Sprintf("-a %s %s:%d -r %s:%d -m -w %d\n", proto, VirtualIP, port, ip, tgtPort, DefaultWeight)
		strBuilder.WriteString(ep)
	}
	return strBuilder.String()
}
