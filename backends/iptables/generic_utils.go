package iptables

import (
	"bytes"
	"fmt"
	"net"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"
	"sigs.k8s.io/kpng/pkg/api/localnetv1"
)

const (
	// IPv4ZeroCIDR is the CIDR block for the whole IPv4 address space
	IPv4ZeroCIDR = "0.0.0.0/0"

	// IPv6ZeroCIDR is the CIDR block for the whole IPv6 address space
	IPv6ZeroCIDR = "::/0"
)

// IsZeroCIDR checks whether the input CIDR string is either
// the IPv4 or IPv6 zero CIDR
func IsZeroCIDR(cidr string) bool {
	if cidr == IPv4ZeroCIDR || cidr == IPv6ZeroCIDR {
		return true
	}
	return false
}

// WriteLine join all words with spaces, terminate with newline and write to buff.
func WriteLine(buf *bytes.Buffer, words ...string) {
	// We avoid strings.Join for performance reasons.
	for i := range words {
		buf.WriteString(words[i])
		if i < len(words)-1 {
			buf.WriteByte(' ')
		} else {
			buf.WriteByte('\n')
		}
	}
}

// WriteBytesLine write bytes to buffer, terminate with newline
func WriteBytesLine(buf *bytes.Buffer, bytes []byte) {
	buf.Write(bytes)
	buf.WriteByte('\n')
}

// WriteRuleLine prepends the strings "-A" and chainName to the buffer and calls
// WriteLine to join all the words into the buffer and terminate with newline.
func WriteRuleLine(buf *bytes.Buffer, chainName string, words ...string) {
	if len(words) == 0 {
		return
	}
	buf.WriteString("-A ")
	buf.WriteString(chainName)
	buf.WriteByte(' ')
	WriteLine(buf, words...)
}

// RevertPorts is closing ports in replacementPortsMap but not in originalPortsMap. In other words, it only
// closes the ports opened in this sync.
func RevertPorts(replacementPortsMap, originalPortsMap map[utilnet.LocalPort]utilnet.Closeable) {
	for k, v := range replacementPortsMap {
		// Only close newly opened local ports - leave ones that were open before this update
		if originalPortsMap[k] == nil {
			klog.V(2).Infof("Closing local port %s", k.String())
			v.Close()
		}
	}
}

// GetLocalAddrs returns a list of all network addresses on the local system
func GetLocalAddrs() ([]net.IP, error) {
	var localAddrs []net.IP

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			return nil, err
		}

		localAddrs = append(localAddrs, ip)
	}

	return localAddrs, nil
}

// GetLocalAddrSet return a local IPSet.
// If failed to get local addr, will assume no local ips.
func GetLocalAddrSet() utilnet.IPSet {
	localAddrs, err := GetLocalAddrs()
	if err != nil {
		klog.ErrorS(err, "Failed to get local addresses assuming no local IPs")
	} else if len(localAddrs) == 0 {
		klog.InfoS("No local addresses were found")
	}

	localAddrSet := utilnet.IPSet{}
	localAddrSet.Insert(localAddrs...)
	return localAddrSet
}

// LogAndEmitIncorrectIPVersionEvent logs and emits incorrect IP version event.
func LogAndEmitIncorrectIPVersionEvent(recorder events.EventRecorder, fieldName, fieldValue, svcNamespace, svcName string, svcUID types.UID) {
	errMsg := fmt.Sprintf("%s in %s has incorrect IP version", fieldValue, fieldName)
	klog.Errorf("%s (service %s/%s).", errMsg, svcNamespace, svcName)
	if recorder != nil {
		recorder.Eventf(
			&v1.ObjectReference{
				Kind:      "Service",
				Name:      svcName,
				Namespace: svcNamespace,
				UID:       svcUID,
			},nil, v1.EventTypeWarning, "KubeProxyIncorrectIPVersion", "GatherEndpoints", errMsg)
	}
}

// GetNodeAddresses return all matched node IP addresses based on given cidr slice.
// Some callers, e.g. IPVS proxier, need concrete IPs, not ranges, which is why this exists.
// NetworkInterfacer is injected for test purpose.
// We expect the cidrs passed in is already validated.
// Given an empty input `[]`, it will return `0.0.0.0/0` and `::/0` directly.
// If multiple cidrs is given, it will return the minimal IP sets, e.g. given input `[1.2.0.0/16, 0.0.0.0/0]`, it will
// only return `0.0.0.0/0`.
// NOTE: GetNodeAddresses only accepts CIDRs, if you want concrete IPs, e.g. 1.2.3.4, then the input should be 1.2.3.4/32.
func GetNodeAddresses(cidrs []string, nw NetworkInterfacer) (sets.String, error) {
	uniqueAddressList := sets.NewString()
	if len(cidrs) == 0 {
		uniqueAddressList.Insert(IPv4ZeroCIDR)
		uniqueAddressList.Insert(IPv6ZeroCIDR)
		return uniqueAddressList, nil
	}
	// First round of iteration to pick out `0.0.0.0/0` or `::/0` for the sake of excluding non-zero IPs.
	for _, cidr := range cidrs {
		if IsZeroCIDR(cidr) {
			uniqueAddressList.Insert(cidr)
		}
	}

	itfs, err := nw.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error listing all interfaces from host, error: %v", err)
	}

	// Second round of iteration to parse IPs based on cidr.
	for _, cidr := range cidrs {
		if IsZeroCIDR(cidr) {
			continue
		}

		_, ipNet, _ := net.ParseCIDR(cidr)
		for _, itf := range itfs {
			addrs, err := nw.Addrs(&itf)
			if err != nil {
				return nil, fmt.Errorf("error getting address from interface %s, error: %v", itf.Name, err)
			}

			for _, addr := range addrs {
				if addr == nil {
					continue
				}

				ip, _, err := net.ParseCIDR(addr.String())
				if err != nil {
					return nil, fmt.Errorf("error parsing CIDR for interface %s, error: %v", itf.Name, err)
				}

				if ipNet.Contains(ip) {
					if utilnet.IsIPv6(ip) && !uniqueAddressList.Has(IPv6ZeroCIDR) {
						uniqueAddressList.Insert(ip.String())
					}
					if !utilnet.IsIPv6(ip) && !uniqueAddressList.Has(IPv4ZeroCIDR) {
						uniqueAddressList.Insert(ip.String())
					}
				}
			}
		}
	}

	if uniqueAddressList.Len() == 0 {
		return nil, fmt.Errorf("no addresses found for cidrs %v", cidrs)
	}

	return uniqueAddressList, nil
}

func ConvertToEPSlices(svc *localnetv1.Service, endpoints []*localnetv1.Endpoint) (*discovery.EndpointSlice, *discovery.EndpointSlice) {
	ePSliceV4 := &discovery.EndpointSlice{}
	ePSliceV6 := &discovery.EndpointSlice{}
	ePSliceV4.AddressType = "IPv4"
	ePSliceV6.AddressType = "IPv6"
	for _, ep := range endpoints {
		k8sEP := &discovery.Endpoint{}
		var slice *discovery.EndpointSlice
		var add string
		if len(ep.GetIPs().V4) > 0 {
			slice = ePSliceV4
			add = ep.GetIPs().V4[0]
		} else if len(ep.GetIPs().V6) > 0 {
			slice = ePSliceV6
			add = ep.GetIPs().V6[0]
		} else {
			klog.Errorf("Empty Endpoint")
			continue
		}
		// slice.ClusterName = svc.
		//TODO when would there be multiple addresses in EP
		k8sEP.Addresses = []string{add}
		k8sEP.Hostname = &ep.Hostname
		slice.Labels = map[string]string{discovery.LabelServiceName: svc.Name}
		slice.Name = svc.Name
		slice.Namespace = svc.Namespace
		slice.Endpoints = append(slice.Endpoints, *k8sEP)
	}
	var k8sPorts []discovery.EndpointPort
	for _, epPort := range svc.GetPorts() {
		var k8sPort discovery.EndpointPort
		protocol := v1.Protocol(epPort.Protocol.String())
		k8sPort.Protocol = &protocol
		k8sPort.Name = &epPort.Name
		k8sPort.Port = &epPort.TargetPort
		k8sPorts = append(k8sPorts, k8sPort)
	}
	ePSliceV4.Ports = k8sPorts
	ePSliceV6.Ports = k8sPorts
	return ePSliceV4, ePSliceV6
}

func ConvertToService(svc *localnetv1.Service) (*v1.Service, error) {

	k8sSvc := &v1.Service{}
	k8sSvc.Annotations = svc.Annotations
	if svc.ExternalTrafficToLocal {
		k8sSvc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
	} else {
		k8sSvc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeCluster
	}
	//TODO after rebase set ClusterIps as well
	k8sSvc.Spec.ClusterIP = svc.IPs.ClusterIP
	var extIps []string
	//TODO any order to maintain
	k8sSvc.Spec.ExternalIPs = append(append(extIps, svc.IPs.ExternalIPs.V4...), svc.IPs.ExternalIPs.V6...)
	k8sSvc.Labels = svc.Labels
	// k8sSvc.Map=svc.MapIP
	k8sSvc.Name = svc.Name
	k8sSvc.Namespace = svc.Namespace
	k8sSvc.Spec.Ports = ConvertToServicePorts(svc.Ports)
	var err error
	k8sSvc.Spec.Type, err = GetSvcType(svc.Type)
	if err != nil {
		return nil, err
	}
	return k8sSvc, nil
}

func ConvertToServicePorts(ports []*localnetv1.PortMapping) []v1.ServicePort {
	var k8sSvcPorts []v1.ServicePort
	for _, port := range ports {
		var k8sSvcPort v1.ServicePort
		k8sSvcPort.Name = port.Name
		k8sSvcPort.NodePort = port.NodePort
		k8sSvcPort.Port = port.Port
		k8sSvcPort.Protocol = v1.Protocol(port.Protocol.String())
		if port.TargetPortName != "" { //CHECK THE LOGIC
			k8sSvcPort.TargetPort = intstr.FromString(port.TargetPortName)
		} else {
			k8sSvcPort.TargetPort = intstr.FromInt(int(port.TargetPort))
		}
		k8sSvcPorts = append(k8sSvcPorts, k8sSvcPort)
	}
	return k8sSvcPorts
}

func GetSvcType(svcType string) (v1.ServiceType, error) {
	if svcType == "ClusterIP" {
		return v1.ServiceTypeClusterIP, nil
	}
	if svcType == "NodePort" {
		return v1.ServiceTypeNodePort, nil
	}
	if svcType == "LoadBalancer" {
		return v1.ServiceTypeLoadBalancer, nil
	}

	if svcType == "ExternalName" {
		return v1.ServiceTypeExternalName, nil
	}
	return "", fmt.Errorf("Invalid service Type: %s", svcType)
}

// CountBytesLines counts the number of lines in a bytes slice
func CountBytesLines(b []byte) int {
	return bytes.Count(b, []byte{'\n'})
}
