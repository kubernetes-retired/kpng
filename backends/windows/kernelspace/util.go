//go:build windows
// +build windows

/*
Copyright 2018-2022 The Kubernetes Authors.

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

package kernelspace

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/Microsoft/hcsshim"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/events"

	v1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
	localv1 "sigs.k8s.io/kpng/api/localv1"
)

type externalIPInfo struct {
	ip    string
	hnsID string
}

type loadBalancerIngressInfo struct {
	ip    string
	hnsID string
}

func Enum(p v1.Protocol) uint16 {
	if p == v1.ProtocolTCP {
		return 6
	}
	if p == v1.ProtocolUDP {
		return 17
	}
	if p == v1.ProtocolSCTP {
		return 132
	}
	return 0
}

type closeable interface {
	Close() error
}

// Uses mac prefix and IPv4 address to return a mac address
// This ensures mac addresses are unique for proper load balancing
// There is a possibility of MAC collisions but this Mac address is used for remote windowsEndpoint only
// and not sent on the wire.
func conjureMac(macPrefix string, ip net.IP) string {
	if ip4 := ip.To4(); ip4 != nil {
		a, b, c, d := ip4[0], ip4[1], ip4[2], ip4[3]
		return fmt.Sprintf("%v-%02x-%02x-%02x-%02x", macPrefix, a, b, c, d)
	} else if ip6 := ip.To16(); ip6 != nil {
		a, b, c, d := ip6[15], ip6[14], ip6[13], ip6[12]
		return fmt.Sprintf("%v-%02x-%02x-%02x-%02x", macPrefix, a, b, c, d)
	}
	return "02-11-22-33-44-55"
}

// KernelCompatTester tests whether the required kernel capabilities are
// present to run the windows kernel proxier.
type KernelCompatTester interface {
	IsCompatible() error
}

// CanUseWinKernelProxier returns true if we should use the Kernel Proxier
// instead of the "classic" userspace Proxier.  This is determined by checking
// the windows kernel version and for the existence of kernel features.
func CanUseWinKernelProxier(kcompat KernelCompatTester) (bool, error) {
	// Check that the kernel supports what we need.
	if err := kcompat.IsCompatible(); err != nil {
		return false, err
	}
	return true, nil
}

type WindowsKernelCompatTester struct{}

// IsCompatible returns true if winkernel can support this mode of proxy
func (lkct WindowsKernelCompatTester) IsCompatible() error {
	_, err := hcsshim.HNSListPolicyListRequest()
	if err != nil {
		return fmt.Errorf("Windows kernel is not compatible for Kernel mode")
	}
	return nil
}

// random util functions fromupstream endpoints.go

// IPPart returns just the IP part of an IP or IP:port or endpoint string. If the IP
// part is an IPv6 address enclosed in brackets (e.g. "[fd00:1::5]:9999"),
// then the brackets are stripped as well.
func IPPart(s string) string {
	if ip := netutils.ParseIPSloppy(s); ip != nil {
		// IP address without port
		return s
	}
	// Must be IP:port
	host, _, err := net.SplitHostPort(s)
	if err != nil {
		klog.ErrorS(err, "Failed to parse host-port", "input", s)
		return ""
	}
	// Check if host string is a valid IP address
	ip := netutils.ParseIPSloppy(host)
	if ip == nil {
		klog.ErrorS(nil, "Failed to parse IP", "input", host)
		return ""
	}
	return ip.String()
}

// PortPart returns just the port part of an endpoint string.
func PortPart(s string) (int, error) {
	// Must be IP:port
	_, port, err := net.SplitHostPort(s)
	if err != nil {
		klog.ErrorS(err, "Failed to parse host-port", "input", s)
		return -1, err
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		klog.ErrorS(err, "Failed to parse port", "input", port)
		return -1, err
	}
	return portNumber, nil
}

const (
	// IPv4ZeroCIDR is the CIDR block for the whole IPv4 address space
	IPv4ZeroCIDR = "0.0.0.0/0"

	// IPv6ZeroCIDR is the CIDR block for the whole IPv6 address space
	IPv6ZeroCIDR = "::/0"
)

var (
	// ErrAddressNotAllowed indicates the address is not allowed
	ErrAddressNotAllowed = errors.New("address not allowed")

	// ErrNoAddresses indicates there are no addresses for the hostname
	ErrNoAddresses = errors.New("No addresses for hostname")
)

// NetworkInterfacer defines an interface for several net library functions. Production
// code will forward to net library functions, and unit tests will override the methods
// for testing purposes.
type NetworkInterfacer interface {
	Addrs(intf *net.Interface) ([]net.Addr, error)
	Interfaces() ([]net.Interface, error)
}

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
func RevertPorts(replacementPortsMap, originalPortsMap map[netutils.LocalPort]netutils.Closeable) {
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
func GetLocalAddrSet() netutils.IPSet {
	localAddrs, err := GetLocalAddrs()
	if err != nil {
		klog.ErrorS(err, "Failed to get local addresses assuming no local IPs")
	} else if len(localAddrs) == 0 {
		klog.InfoS("No local addresses were found")
	}

	localAddrSet := netutils.IPSet{}
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
			}, nil, v1.EventTypeWarning, "KubeProxyIncorrectIPVersion", "GatherEndpoints", errMsg)
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
					if netutils.IsIPv6(ip) && !uniqueAddressList.Has(IPv6ZeroCIDR) {
						uniqueAddressList.Insert(ip.String())
					}
					if !netutils.IsIPv6(ip) && !uniqueAddressList.Has(IPv4ZeroCIDR) {
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

// GetClusterIPByFamily returns a service clusterip by family
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

// RequestsOnlyLocalTraffic checks if service requests OnlyLocal traffic.
func RequestsOnlyLocalTraffic(service *localv1.Service) bool {
	if service.Type != string(v1.ServiceTypeLoadBalancer) &&
		service.Type != string(v1.ServiceTypeNodePort) {
		return false
	}
	return service.ExternalTrafficToLocal
}

// MapIPsByIPFamily maps a slice of IPs to their respective IP families (v4 or v6)
func MapIPsByIPFamily(ips *localv1.IPSet) map[v1.IPFamily][]string {
	ipFamilyMap := map[v1.IPFamily][]string{}
	ipFamilyMap[v1.IPv4Protocol] = append(ipFamilyMap[v1.IPv4Protocol], ips.V4...)
	ipFamilyMap[v1.IPv6Protocol] = append(ipFamilyMap[v1.IPv6Protocol], ips.V6...)
	return ipFamilyMap
}

func getIPFamilyFromIP(ipStr string) (v1.IPFamily, error) {
	netIP := net.ParseIP(ipStr)
	if netIP == nil {
		return "", ErrAddressNotAllowed
	}

	if netutils.IsIPv6(netIP) {
		return v1.IPv6Protocol, nil
	}
	return v1.IPv4Protocol, nil
}

// OtherIPFamily returns the other ip family
func OtherIPFamily(ipFamily v1.IPFamily) v1.IPFamily {
	if ipFamily == v1.IPv6Protocol {
		return v1.IPv4Protocol
	}

	return v1.IPv6Protocol
}

// MapCIDRsByIPFamily maps a slice of IPs to their respective IP families (v4 or v6)
func MapCIDRsByIPFamily(cidrStrings []string) map[v1.IPFamily][]string {
	ipFamilyMap := map[v1.IPFamily][]string{}
	for _, cidr := range cidrStrings {
		// Handle only the valid CIDRs
		if ipFamily, err := getIPFamilyFromCIDR(cidr); err == nil {
			ipFamilyMap[ipFamily] = append(ipFamilyMap[ipFamily], cidr)
		} else {
			klog.Errorf("Skipping invalid cidr: %s", cidr)
		}
	}
	return ipFamilyMap
}

func getIPFamilyFromCIDR(cidrStr string) (v1.IPFamily, error) {
	_, netCIDR, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return "", ErrAddressNotAllowed
	}
	if netutils.IsIPv6CIDR(netCIDR) {
		return v1.IPv6Protocol, nil
	}
	return v1.IPv4Protocol, nil
}

// CountBytesLines counts the number of lines in a bytes slice
func CountBytesLines(b []byte) int {
	return bytes.Count(b, []byte{'\n'})
}
