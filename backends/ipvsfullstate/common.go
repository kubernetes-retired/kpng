package ipvsfullsate

import (
	"github.com/google/seesaw/ipvs"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
	"net"
	"sigs.k8s.io/kpng/api/localv1"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvsfullstate/util"
	"strings"
)

const (
	// FlagPersistent specify IPVS service session affinity
	FlagPersistent = 0x1
	// FlagHashed specify IPVS service hash flag
	FlagHashed = 0x2
)

var protocolIPSetMap = map[string]string{
	ipsetutil.ProtocolTCP:  kubeNodePortSetTCP,
	ipsetutil.ProtocolUDP:  kubeNodePortSetUDP,
	ipsetutil.ProtocolSCTP: kubeNodePortSetSCTP,
}

type Operation int32

const (
	AddService     Operation = 0
	DeleteService  Operation = 1
	AddEndPoint    Operation = 2
	DeleteEndPoint Operation = 3
)

func asDummyIPs(ip string, ipFamily v1.IPFamily) string {
	if ipFamily == v1.IPv4Protocol {
		return ip + "/32"
	}

	if ipFamily == v1.IPv6Protocol {
		return ip + "/128"
	}
	return ip + "/32"
}

func interfaceAddresses() []string {
	ifacesAddress, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	var addresses []string
	for _, addr := range ifacesAddress {
		// TODO: Ignore interfaces in PodCIDR or ClusterCIDR
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			panic(err)
		}
		// I want to deal only with IPv4 right now...
		if ipv4 := ip.To4(); ipv4 == nil {
			continue
		}

		addresses = append(addresses, ip.String())
	}
	return addresses
}

func GetIPFamily(ipAddr string) v1.IPFamily {
	var ipAddrFamily v1.IPFamily
	if netutils.IsIPv4String(ipAddr) {
		ipAddrFamily = v1.IPv4Protocol
	}

	if netutils.IsIPv6String(ipAddr) {
		ipAddrFamily = v1.IPv6Protocol
	}
	return ipAddrFamily
}

// GetClusterIPByFamily returns a service clusterIP by family
func GetClusterIPByFamily(ipFamily v1.IPFamily, service *localv1.Service) string {
	if ipFamily == v1.IPv4Protocol {
		if len(service.IPs.ClusterIPs.V4) > 0 {
			return service.IPs.ClusterIPs.V4[0]
		}
	}
	if ipFamily == v1.IPv6Protocol {
		if len(service.IPs.ClusterIPs.V6) > 0 {
			return service.IPs.ClusterIPs.V6[0]
		}
	}
	return ""
}

func (p *proxier) addVirtualServer(baseServicePort *ServicePortInfo) {
	vs := baseServicePort.GetVirtualServer()

	klog.V(2).Infof("adding AddVirtualServer: port: %v", baseServicePort)
	// Programme virtual-server directly
	ipvsSvc := vs.ToService()
	err := ipvs.AddService(ipvsSvc)
	if err != nil && !strings.HasSuffix(err.Error(), "object exists") {
		klog.Error("failed to add service in IPVS", ": ", err)
	}
	p.addOrDelClusterIPInIPSet(baseServicePort, AddService)

}

func (p *proxier) deleteVirtualServer(baseServicePort *ServicePortInfo) {
	klog.V(2).Infof("deleting service , serviceIP (%v) , port (%v)", baseServicePort.serviceIP, baseServicePort.Port())
	err := ipvs.DeleteService(baseServicePort.GetVirtualServer().ToService())
	if err != nil {
		klog.Error("failed to delete service from IPVS", baseServicePort.serviceIP, ": ", err)
	}
	p.addOrDelClusterIPInIPSet(baseServicePort, DeleteService)
}

func (p *proxier) addOrDelClusterIPInIPSet(port *ServicePortInfo, op Operation) {
	// Capture the clusterIP.
	entry := getIPSetEntry("", port)
	// add service Cluster IP:Port to kubeServiceAccess ip set for the purpose of solving hairpin.
	if valid := p.ipsetList[kubeClusterIPSet].validateEntry(entry); !valid {
		klog.Errorf("error adding entry :%s, to ipset:%s", entry.String(), p.ipsetList[kubeClusterIPSet].Name)
		return
	}
	set := p.ipsetList[kubeClusterIPSet]
	if op == AddService {
		if err := set.handle.AddEntry(entry.String(), &set.IPSet, true); err != nil {
			klog.Errorf("Failed to add entry %v into ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully add entry: %v into ip set: %s", entry, set.Name)
		}

	}
	if op == DeleteService {
		if err := set.handle.DelEntry(entry.String(), set.Name); err != nil {
			klog.Errorf("Failed to delete entry: %v from ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully deleted entry: %v to ip set: %s", entry, set.Name)
		}

	}
}

func getEndPointEntry(endPointIP, protocol string, targetPort int32) *ipsetutil.Entry {
	return &ipsetutil.Entry{
		IP:       endPointIP,
		Port:     int(targetPort),
		Protocol: strings.ToLower(protocol),
		IP2:      endPointIP,
		SetType:  ipsetutil.HashIPPortIP,
	}
}

func getIPSetEntry(srcAddr string, port *ServicePortInfo) *ipsetutil.Entry {
	if srcAddr != "" {
		return &ipsetutil.Entry{
			IP:       port.ServiceIP(),
			Port:     int(port.Port()),
			Protocol: strings.ToLower(port.Protocol().String()),
			SetType:  ipsetutil.HashIPPort,
			Net:      srcAddr,
		}
	}
	return &ipsetutil.Entry{
		IP:       port.ServiceIP(),
		Port:     int(port.Port()),
		Protocol: strings.ToLower(port.Protocol().String()),
		SetType:  ipsetutil.HashIPPort,
	}
}

func (p *proxier) addOrDelEndPointInIPSet(endPointIP, protocol string, targetPort int32, isLocalEndPoint bool, op Operation) {
	if !isLocalEndPoint {
		return
	}

	entry := getEndPointEntry(endPointIP, protocol, targetPort)
	set := p.ipsetList[kubeLoopBackIPSet]
	if valid := set.validateEntry(entry); !valid {
		klog.Errorf("error adding entry to ipset. entry:%s, ipset:%s", entry.String(), p.ipsetList[kubeLoopBackIPSet].Name)
		return
	}
	if op == AddEndPoint {
		if err := set.handle.AddEntry(entry.String(), &set.IPSet, true); err != nil {
			klog.Errorf("Failed to add entry %v into ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully add entry: %v into ip set: %s", entry, set.Name)
		}

	}
	if op == DeleteEndPoint {
		if err := set.handle.DelEntry(entry.String(), set.Name); err != nil {
			klog.Errorf("Failed to delete entry: %v from ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully deleted entry: %v to ip set: %s", entry, set.Name)
		}

	}
}

func (p *proxier) addRealServer(baseServicePort *ServicePortInfo, endpointInfo *EndPointInfo) {
	destination := ipvsSvcDst{
		Svc: baseServicePort.GetVirtualServer().ToService(),
		Dst: ipvsDestination(*endpointInfo, baseServicePort),
	}
	klog.V(2).Infof("adding destination ep (%v)", endpointInfo.endPointIP)
	if err := ipvs.AddDestination(destination.Svc, destination.Dst); err != nil && !strings.HasSuffix(err.Error(), "object exists") {
		klog.Error("failed to add destination : ", err)
	}

	p.addOrDelEndPointInIPSet(
		endpointInfo.endPointIP,
		baseServicePort.Protocol().String(),
		baseServicePort.targetPort,
		endpointInfo.isLocalEndPoint,
		AddEndPoint)
}

func (p *proxier) deleteRealServer(baseServicePort *ServicePortInfo, endpointInfo *EndPointInfo) {

	vs := baseServicePort.GetVirtualServer()
	klog.V(2).Infof("deleteRealServer, portInfo : %v", endpointInfo)
	dest := ipvsSvcDst{
		Svc: vs.ToService(),
		Dst: ipvsDestination(*endpointInfo, baseServicePort),
	}

	klog.V(2).Infof("deleting destination : %v", dest)
	if err := ipvs.DeleteDestination(dest.Svc, dest.Dst); err != nil {
		klog.Error("failed to delete destination ", dest, ": ", err)
	}

	p.addOrDelEndPointInIPSet(
		endpointInfo.endPointIP,
		baseServicePort.Protocol().String(),
		baseServicePort.targetPort,
		endpointInfo.isLocalEndPoint,
		DeleteEndPoint)

}
