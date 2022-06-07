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
	"errors"
	"strconv"
	"strings"

	"github.com/google/seesaw/ipvs"
	"sigs.k8s.io/kpng/client/lightdiffstore"
	"sigs.k8s.io/kpng/client/serviceevents"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"

	"sigs.k8s.io/kpng/api/localnetv1"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
)

const (
	ClusterIPService    = "ClusterIP"
	NodePortService     = "NodePort"
	LoadBalancerService = "LoadBalancer"
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

type endPointInfo struct {
	endPointIP      string
	isLocalEndPoint bool
	portMap         map[string]int32
}

func asDummyIPs(ip string, ipFamily v1.IPFamily) string {
	if ipFamily == v1.IPv4Protocol {
		return ip + "/32"
	}

	if ipFamily == v1.IPv6Protocol {
		return ip + "/128"
	}
	return ip + "/32"
}

func epPortSuffix(port *localnetv1.PortMapping) string {
	return port.Protocol.String() + ":" + strconv.Itoa(int(port.Port))
}

func getServiceKey(svc *localnetv1.Service) string {
	return svc.Namespace + "/" + svc.Name
}

func getServicePortKey(serviceKey, serviceIP string, port *localnetv1.PortMapping) string {
	return serviceKey + "/" + serviceIP + "/" + epPortSuffix(port)
}

func getPortKey(serviceKey string, port *localnetv1.PortMapping) string {
	return serviceKey + "/" + epPortSuffix(port)
}

// Any service event (ip or port update) received after
// EP creation is considered as service update. Else
// its a new service creation.
func (s *Backend) isServiceUpdated(serviceKey string) bool {
	if s.svcEPMap[serviceKey] >= 1 {
		return true
	}
	return false
}

func (p *proxier) AddOrDelEndPointInIPSet(endPointIP, protocol string, targetPort int32, isLocalEndPoint bool, op Operation) {
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
		p.updateRefCountForIPSet(kubeLoopBackIPSet, op)
	}
	if op == DeleteEndPoint {
		if err := set.handle.DelEntry(entry.String(), set.Name); err != nil {
			klog.Errorf("Failed to delete entry: %v from ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully deleted entry: %v to ip set: %s", entry, set.Name)
		}
		p.updateRefCountForIPSet(kubeLoopBackIPSet, op)
	}
}

func getIPFamily(ipAddr string) v1.IPFamily {
	var ipAddrFamily v1.IPFamily
	if netutils.IsIPv4String(ipAddr) {
		ipAddrFamily = v1.IPv4Protocol
	}

	if netutils.IsIPv6String(ipAddr) {
		ipAddrFamily = v1.IPv6Protocol
	}
	return ipAddrFamily
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

func (p *proxier) AddOrDelClusterIPInIPSet(port *BaseServicePortInfo, op Operation) {
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
		//Increment ref count for clusterIP
		p.updateRefCountForIPSet(kubeClusterIPSet, op)
	}
	if op == DeleteService {
		if err := set.handle.DelEntry(entry.String(), set.Name); err != nil {
			klog.Errorf("Failed to delete entry: %v from ip set: %s, error: %v", entry, set.Name, err)
		} else {
			klog.V(3).Infof("Successfully deleted entry: %v to ip set: %s", entry, set.Name)
		}
		//Increment ref count for clusterIP
		p.updateRefCountForIPSet(kubeClusterIPSet, op)
	}
}

func (p *proxier) addRealServerForPort(serviceKey string, portList []*BaseServicePortInfo) []endPointInfo {
	var epList []endPointInfo
	for _, port := range portList {
		klog.V(2).Infof("addRealServerForPort port (%v)", port)
		for _, epKV := range p.endpoints.GetByPrefix([]byte(serviceKey)) {
			epInfo := epKV.Value.(endPointInfo)
			epList = append(epList, epInfo)
			destination := ipvsSvcDst{
				Svc: port.GetVirtualServer().ToService(),
				Dst: ipvsDestination(epInfo, port),
			}
			klog.V(2).Infof("adding destination ep (%v)", epInfo.endPointIP)
			if err := ipvs.AddDestination(destination.Svc, destination.Dst); err != nil && !strings.HasSuffix(err.Error(), "object exists") {
				klog.Error("failed to add destination ", serviceKey, ": ", err)
			}
		}
	}
	return epList
}

func (p *proxier) deleteRealServerForPort(serviceKey string, portList []*BaseServicePortInfo) []endPointInfo {
	var epList []endPointInfo
	for _, port := range portList {
		klog.V(2).Infof("deleteRealServerForPort port (%v)", port)
		for _, epKV := range p.endpoints.GetByPrefix([]byte(serviceKey)) {
			epInfo := epKV.Value.(endPointInfo)
			epList = append(epList, epInfo)
			destination := ipvsSvcDst{
				Svc: port.GetVirtualServer().ToService(),
				Dst: ipvsDestination(epInfo, port),
			}
			klog.V(2).Infof("deleting destination ep (%v)", epInfo.endPointIP)
			if err := ipvs.DeleteDestination(destination.Svc, destination.Dst); err != nil && !strings.HasSuffix(err.Error(), "object exists") {
				klog.Error("failed to delete destination ", serviceKey, ": ", err)
			}
		}
	}
	return epList
}

func getIPSetEntry(srcAddr string, port *BaseServicePortInfo) *ipsetutil.Entry {
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

func getIPFamiliesOfService(svc *localnetv1.Service) []v1.IPFamily {
	var ipFamilies []v1.IPFamily
	if len(svc.IPs.ClusterIPs.V4) != 0 {
		ipFamilies = append(ipFamilies, v1.IPv4Protocol)
	}
	if len(svc.IPs.ClusterIPs.V6) != 0 {
		ipFamilies = append(ipFamilies, v1.IPv6Protocol)
	}
	return ipFamilies
}

func (p *proxier) getLbIPForIPFamily(svc *localnetv1.Service) (error, string) {
	var svcIP string
	if svc.IPs.LoadBalancerIPs == nil {
		return errors.New("LB IPs are not configured"), svcIP
	}

	if p.ipFamily == v1.IPv4Protocol && len(svc.IPs.LoadBalancerIPs.V4) > 0 {
		svcIP = svc.IPs.LoadBalancerIPs.V4[0]
	}
	if p.ipFamily == v1.IPv6Protocol && len(svc.IPs.LoadBalancerIPs.V6) > 0 {
		svcIP = svc.IPs.LoadBalancerIPs.V6[0]
	}
	return nil, svcIP
}

func (p *proxier) addVirtualServer(portInfo *BaseServicePortInfo) {
	vs := portInfo.GetVirtualServer()

	klog.V(2).Infof("adding AddVirtualServer: port: %v", portInfo)
	// Programme virtual-server directly
	ipvsSvc := vs.ToService()
	err := ipvs.AddService(ipvsSvc)
	if err != nil && !strings.HasSuffix(err.Error(), "object exists") {
		klog.Error("failed to add service in IPVS", ": ", err)
	}
}

func (p *proxier) deleteVirtualServer(portInfo *BaseServicePortInfo) {
	klog.V(2).Infof("deleting service , serviceIP (%v) , port (%v)", portInfo.serviceIP, portInfo.Port())
	err := ipvs.DeleteService(portInfo.GetVirtualServer().ToService())
	if err != nil {
		klog.Error("failed to delete service from IPVS", portInfo.serviceIP, ": ", err)
	}
}

func (p *proxier) AddOrDelNodePortInIPSet(port *localnetv1.PortMapping, op Operation) {
	var entries []*ipsetutil.Entry
	protocol := strings.ToLower(port.Protocol.String())
	ipSetName := protocolIPSetMap[protocol]
	p.updateRefCountForIPSet(ipSetName, op)
	nodePortSet := p.ipsetList[ipSetName]
	switch protocol {
	case ipsetutil.ProtocolTCP, ipsetutil.ProtocolUDP:
		entries = []*ipsetutil.Entry{getNodePortIPSetEntry(int(port.NodePort), protocol, ipsetutil.BitmapPort)}

	case ipsetutil.ProtocolSCTP:
		// Since hash ip:port is used for SCTP, all the nodeIPs to be used in the SCTP ipset entries.
		entries = []*ipsetutil.Entry{}
		for _, nodeIP := range p.nodeAddresses {
			entry := getNodePortIPSetEntry(int(port.NodePort), protocol, ipsetutil.HashIPPort)
			entry.IP = nodeIP
			entries = append(entries, entry)
		}
	default:
		// It should never hit
		klog.Errorf("Unsupported protocol type %v protocol ", protocol)
	}
	if nodePortSet != nil {
		for _, entry := range entries {
			if valid := nodePortSet.validateEntry(entry); !valid {
				klog.Errorf("error adding entry (%v) to ipset (%v)", entry.String(), nodePortSet.Name)
			}
			set := p.ipsetList[ipSetName]
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
	}
}

func getNodePortIPSetEntry(port int, protocol string, ipSetType ipsetutil.Type) *ipsetutil.Entry {
	return &ipsetutil.Entry{
		// No need to provide ip info
		Port:     port,
		Protocol: protocol,
		SetType:  ipSetType,
	}
}

func (p *proxier) sync() {
	// sync iptable rules
	p.syncIPTableRules()

	// signal diffstores we've finished
	p.endpoints.Reset(lightdiffstore.ItemUnchanged)
}

func (p *proxier) updateRefCountForIPSet(setName string, op Operation) {
	if op == AddService || op == AddEndPoint {
		p.ipsetList[setName].refCountOfSvc++
	}
	if op == DeleteService || op == DeleteEndPoint {
		p.ipsetList[setName].refCountOfSvc--
	}
}

func (s *Backend) enableSessionAffinityForServiceIPs(svc *localnetv1.Service, sessionAffinity serviceevents.SessionAffinity) {
	serviceKey := getServiceKey(svc)
	ipFamilies := getIPFamiliesOfService(svc)
	for _, ipFamily := range ipFamilies {
		s.proxiers[ipFamily].enableSessionAffinityForServiceIP(serviceKey, sessionAffinity)
	}
}

func (s *Backend) disableSessionAffinityForServiceIPs(svc *localnetv1.Service) {
	serviceKey := getServiceKey(svc)
	ipFamilies := getIPFamiliesOfService(svc)
	for _, ipFamily := range ipFamilies {
		s.proxiers[ipFamily].disableSessionAffinityForServiceIP(serviceKey)
	}
}

func (p *proxier) enableSessionAffinityForServiceIP(serviceKey string, sa serviceevents.SessionAffinity) {
	for _, sp := range p.servicePorts.GetByPrefix([]byte(serviceKey)) {
		portInfo := sp.Value.(BaseServicePortInfo)
		// Set session affinity for port
		portInfo.SetSessionAffinity(sa)

		vs := portInfo.GetVirtualServer()
		// Programme virtual-server directly
		ipvsSvc := vs.ToService()
		err := ipvs.UpdateService(ipvsSvc)
		if err != nil && !strings.HasSuffix(err.Error(), "object exists") {
			klog.Error("failed to add service in IPVS", serviceKey, ": ", err)
		}
		klog.V(2).Infof("enable sess-aff ipvsSvc: %v", ipvsSvc)
		p.servicePorts.Set([]byte(serviceKey), 0, portInfo)
	}
}

func (p *proxier) disableSessionAffinityForServiceIP(serviceKey string) {
	for _, sp := range p.servicePorts.GetByPrefix([]byte(serviceKey)) {
		portInfo := sp.Value.(BaseServicePortInfo)
		portInfo.ResetSessionAffinity()
		vs := portInfo.GetVirtualServer()

		// Programme virtual-server directly
		ipvsSvc := vs.ToService()
		err := ipvs.UpdateService(ipvsSvc)
		if err != nil && !strings.HasSuffix(err.Error(), "object exists") {
			klog.Error("failed to add service in IPVS", serviceKey, ": ", err)
		}
		klog.V(2).Infof("disable sess-aff : %v", ipvsSvc)
		p.servicePorts.Set([]byte(serviceKey), 0, portInfo)
	}
}

func (p *proxier) addRealServer(serviceKey, prefix, endPointIP string, endpoint *localnetv1.Endpoint) {
	epInfo := endPointInfo{
		endPointIP:      endPointIP,
		isLocalEndPoint: endpoint.Local,
		portMap:         make(map[string]int32),
	}

	for _, port := range endpoint.PortOverrides {
		epInfo.portMap[port.Name] = port.Port
	}
	p.endpoints.Set([]byte(prefix), 0, epInfo)
	for _, sp := range p.servicePorts.GetByPrefix([]byte(serviceKey)) {
		portInfo := sp.Value.(BaseServicePortInfo)
		klog.V(2).Infof("addRealServer, portInfo : %v", portInfo)
		vs := portInfo.GetVirtualServer()
		dest := ipvsSvcDst{
			Svc: vs.ToService(),
			Dst: ipvsDestination(epInfo, &portInfo),
		}
		klog.V(2).Infof("adding destination ep (%v)", endPointIP)
		if err := ipvs.AddDestination(dest.Svc, dest.Dst); err != nil && !strings.HasSuffix(err.Error(), "object exists") {
			klog.Error("failed to add destination ", dest, ": ", err)
		}
	}
	portList := p.portMap[serviceKey]
	klog.V(2).Infof("addRealServer, portList : %v", portList)
	for _, port := range portList {
		p.AddOrDelEndPointInIPSet(endPointIP, port.Protocol.String(), port.TargetPort, endpoint.Local, AddEndPoint)
	}
}

func (p *proxier) deleteRealServer(serviceKey, prefix string) {
	for _, kv := range p.endpoints.GetByPrefix([]byte(prefix)) {
		epInfo := kv.Value.(endPointInfo)
		for _, sp := range p.servicePorts.GetByPrefix([]byte(serviceKey)) {
			portInfo := sp.Value.(BaseServicePortInfo)
			vs := portInfo.GetVirtualServer()
			klog.V(2).Infof("deleteRealServer, portInfo : %v", portInfo)
			dest := ipvsSvcDst{
				Svc: vs.ToService(),
				Dst: ipvsDestination(epInfo, &portInfo),
			}

			klog.V(2).Infof("deleting destination : %v", dest)
			if err := ipvs.DeleteDestination(dest.Svc, dest.Dst); err != nil {
				klog.Error("failed to delete destination ", dest, ": ", err)
			}
		}
		portList := p.portMap[serviceKey]
		klog.V(2).Infof("deleteRealServer, portList : %v", portList)
		for _, port := range portList {
			p.AddOrDelEndPointInIPSet(epInfo.endPointIP, port.Protocol.String(), port.TargetPort, epInfo.isLocalEndPoint, DeleteEndPoint)
		}
	}

	// remove this endpoint from the endpoints
	p.endpoints.DeleteByPrefix([]byte(prefix))
}

func (p *proxier) deletePortFromPortMap(serviceKey, portMapKey string) {
	klog.V(2).Infof("deletePortFromPortMap, portMapKey= %v, portMap=%+v", portMapKey, p.portMap[serviceKey])
	delete(p.portMap[serviceKey], portMapKey)
	if len(p.portMap[serviceKey]) == 0 {
		delete(p.portMap, serviceKey)
		klog.V(2).Infof("deletePortFromPortMap, svcKey=%v", serviceKey)
	}
}
