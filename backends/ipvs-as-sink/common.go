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
	"github.com/google/seesaw/ipvs"
	"sigs.k8s.io/kpng/client/pkg/diffstore"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	netutils "k8s.io/utils/net"

	"sigs.k8s.io/kpng/api/localnetv1"
	ipsetutil "sigs.k8s.io/kpng/backends/ipvs-as-sink/util"
)

type endPointInfo struct {
	endPointIP      string
	isLocalEndPoint bool
}

// Any service event (ip or port update) received after
// EP creation is considered as service update. Else
// its a new service creation.
func (s *Backend) isServiceUpdated(svc *localnetv1.Service) bool {
	serviceKey := svc.Namespace + "/" + svc.Name
	if s.svcEPMap[serviceKey] >=1 {
		return true
	}
	return false
}

func (p *proxier) AddOrDelEndPointInIPSet(endPointList []string, portList []*localnetv1.PortMapping, isLocalEndPoint bool, op Operation) {
	if !isLocalEndPoint {
		return
	}
	for _, port := range portList {
		for _, endPointIP := range endPointList {
			entry := getEndPointEntry(endPointIP, port)
			if valid := p.ipsetList[kubeLoopBackIPSet].validateEntry(entry); !valid {
				klog.Errorf("error adding entry to ipset. entry:%s, ipset:%s", entry.String(), p.ipsetList[kubeLoopBackIPSet].Name)
				return
			}
			if op == AddEndPoint {
				p.ipsetList[kubeLoopBackIPSet].newEntries.Insert(entry.String())
				p.updateRefCountForIPSet(kubeLoopBackIPSet, op)
			}
			if op == DeleteEndPoint {
				p.ipsetList[kubeLoopBackIPSet].deleteEntries.Insert(entry.String())
				p.updateRefCountForIPSet(kubeLoopBackIPSet, op)
			}
		}
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

func getEndPointEntry(endPointIP string, port *localnetv1.PortMapping) *ipsetutil.Entry {
	return &ipsetutil.Entry{
		IP:       endPointIP,
		Port:     int(port.TargetPort),
		Protocol: strings.ToLower(port.Protocol.String()),
		IP2:      endPointIP,
		SetType:  ipsetutil.HashIPPortIP,
	}
}

func (p *proxier) AddOrDelClusterIPInIPSet(clusterIP  string, portList []*localnetv1.PortMapping, op Operation) {
	for _, port := range portList {
		// Capture the clusterIP.
		entry := getIPSetEntry(clusterIP, "", port)
		// add service Cluster IP:Port to kubeServiceAccess ip set for the purpose of solving hairpin.
		if valid := p.ipsetList[kubeClusterIPSet].validateEntry(entry); !valid {
			klog.Errorf("error adding entry :%s, to ipset:%s", entry.String(), p.ipsetList[kubeClusterIPSet].Name)
			return
		}
		if op == AddService {
			p.ipsetList[kubeClusterIPSet].newEntries.Insert(entry.String())
			//Increment ref count for clusterIP
			p.updateRefCountForIPSet(kubeClusterIPSet, op)
		}
		if op == DeleteService {
			p.ipsetList[kubeClusterIPSet].deleteEntries.Insert(entry.String())
			//Increment ref count for clusterIP
			p.updateRefCountForIPSet(kubeClusterIPSet, op)
		}
	}
}

func (p *proxier) updateIPVSDestWithPort(key , clusterIP string, port *localnetv1.PortMapping) ([]string, bool) {
	var endPointList []string
	var isLocalEndPoint bool

	for _, epKV := range p.endpoints.GetByPrefix([]byte(key + "/")) {
		epInfo := epKV.Value.(endPointInfo)
		endPointList = append(endPointList, epInfo.endPointIP)
		isLocalEndPoint = epInfo.isLocalEndPoint

		lbKey := key + "/" + clusterIP + "/" + epPortSuffix(port)
		ipvslb := p.lbs.GetByPrefix([]byte(lbKey))
		p.dests.Set([]byte(lbKey + "/" + epInfo.endPointIP), 0, ipvsSvcDst{
			Svc: ipvslb[0].Value.(ipvsLB).ToService(),
			Dst: ipvsDestination(epInfo.endPointIP, port, p.weight),
		})
	}

	return endPointList, isLocalEndPoint
}

func (p *proxier) deleteIPVSDestForPort(key , clusterIP string, port *localnetv1.PortMapping) ([]string, bool) {
	var endPointList []string
	var isLocalEndPoint bool

	for _, epKV := range p.endpoints.GetByPrefix([]byte(key + "/")) {
		epInfo := epKV.Value.(endPointInfo)
		endPointList = append(endPointList, epInfo.endPointIP)
		isLocalEndPoint = epInfo.isLocalEndPoint
		lbKey := key + "/" + clusterIP + "/" + epPortSuffix(port)
		p.dests.Delete([]byte(lbKey + "/" + epInfo.endPointIP))
	}

	return endPointList, isLocalEndPoint
}

func getIPSetEntry(svcIP,srcAddr string, port *localnetv1.PortMapping) *ipsetutil.Entry {
	if srcAddr != "" {
		return &ipsetutil.Entry{
			IP:       svcIP,
			Port:     int(port.Port),
			Protocol: strings.ToLower(port.Protocol.String()),
			SetType:  ipsetutil.HashIPPort,
			Net: srcAddr,
		}
	}
	return &ipsetutil.Entry{
		IP:       svcIP,
		Port:     int(port.Port),
		Protocol: strings.ToLower(port.Protocol.String()),
		SetType:  ipsetutil.HashIPPort,
	}
}

func getServiceIPForIPFamily(ipFamily v1.IPFamily, svc *localnetv1.Service) string {
	var svcIP string
	if ipFamily == v1.IPv4Protocol {
		svcIP = svc.IPs.ClusterIPs.V4[0]
	}
	if ipFamily == v1.IPv6Protocol {
		svcIP = svc.IPs.ClusterIPs.V6[0]
	}
	return svcIP
}

func (p *proxier) getLbIPForIPFamily(svc *localnetv1.Service) (error, string)  {
	var svcIP string
	if svc.IPs.LoadBalancerIPs == nil {
		return errors.New("LB IPs are not configured"), svcIP
	}

	if p.ipFamily == v1.IPv4Protocol && len(svc.IPs.LoadBalancerIPs.V4) > 0{
		svcIP = svc.IPs.LoadBalancerIPs.V4[0]
	}
	if p.ipFamily == v1.IPv6Protocol && len(svc.IPs.LoadBalancerIPs.V6) > 0{
		svcIP = svc.IPs.LoadBalancerIPs.V6[0]
	}
	return nil, svcIP
}

func (p *proxier) storeLBSvc(port *localnetv1.PortMapping, svcIP , key, svcType string) {
	prefix := key + "/" + svcIP + "/"
	lbKey := prefix + epPortSuffix(port)
	p.lbs.Set([]byte(lbKey), 0, ipvsLB{IP: svcIP, ServiceKey: key, Port: port, SchedulingMethod: p.schedulingMethod, ServiceType: svcType})
}

func (p *proxier) deleteLBSvc(port *localnetv1.PortMapping, svcIP , key string) {
	prefix := key + "/" + svcIP + "/"
	lbKey := prefix + epPortSuffix(port)
	p.lbs.Delete([]byte(lbKey))
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
		klog.Errorf( "Unsupported protocol type %v protocol ", protocol)
	}
	if nodePortSet != nil {
		for _, entry := range entries {
			if valid := nodePortSet.validateEntry(entry); !valid {
				klog.Errorf( "error adding entry (%v) to ipset (%v)", entry.String(),  nodePortSet.Name)
			}
			if op == AddService {
				nodePortSet.newEntries.Insert(entry.String())
			}
			if op == DeleteService {
				nodePortSet.deleteEntries.Insert(entry.String())
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
	// add service IP into IPVS
	for _, lbKV := range p.lbs.Updated() {
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
	for _, kv := range p.dests.Updated() {
		svcDst := kv.Value.(ipvsSvcDst)

		klog.V(2).Info("adding destination ", string(kv.Key))
		if err := ipvs.AddDestination(svcDst.Svc, svcDst.Dst); err != nil && !strings.HasSuffix(err.Error(), "object exists") {
			klog.Error("failed to add destination ", string(kv.Key), ": ", err)
		}
	}

	// Delete endpoint/real-server entries from IPVS
	for _, kv := range p.dests.Deleted() {
		svcDst := kv.Value.(ipvsSvcDst)

		klog.V(2).Info("deleting destination ", string(kv.Key))
		if err := ipvs.DeleteDestination(svcDst.Svc, svcDst.Dst); err != nil {
			klog.Error("failed to delete destination ", string(kv.Key), ": ", err)
		}
	}

	// remove service IP from IPVS
	for _, lbKV := range p.lbs.Deleted() {
		lb := lbKV.Value.(ipvsLB)

		klog.V(2).Info("deleting service ", string(lbKV.Key))
		err := ipvs.DeleteService(lb.ToService())

		if err != nil {
			klog.Error("failed to delete service from IPVS", string(lbKV.Key), ": ", err)
		}
	}

	// sync ipset entries
	for _, set := range p.ipsetList {
		set.syncIPSetEntries()
	}

	// sync iptable rules
	p.syncIPTableRules()


	// reset ipset entries
	for _, set := range p.ipsetList {
		set.resetEntries()
	}

	// signal diffstores we've finished
	p.lbs.Reset(diffstore.ItemUnchanged)
	p.endpoints.Reset(diffstore.ItemUnchanged)
	p.dests.Reset(diffstore.ItemUnchanged)
}

func (p *proxier) updateRefCountForIPSet(setName string, op Operation) {
	if op == AddService || op == AddEndPoint {
		p.ipsetList[setName].refCountOfSvc++
	}
	if op == DeleteService || op == DeleteEndPoint {
		p.ipsetList[setName].refCountOfSvc--
	}
}
