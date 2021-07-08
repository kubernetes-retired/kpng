package iptables

import (
	"bytes"
	"k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"
	"k8s.io/client-go/tools/record"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"net"
	"fmt"
	v1 "k8s.io/api/core/v1"
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
func LogAndEmitIncorrectIPVersionEvent(recorder record.EventRecorder, fieldName, fieldValue, svcNamespace, svcName string, svcUID types.UID) {
	errMsg := fmt.Sprintf("%s in %s has incorrect IP version", fieldValue, fieldName)
	klog.Errorf("%s (service %s/%s).", errMsg, svcNamespace, svcName)
	if recorder != nil {
		recorder.Eventf(
			&v1.ObjectReference{
				Kind:      "Service",
				Name:      svcName,
				Namespace: svcNamespace,
				UID:       svcUID,
			}, v1.EventTypeWarning, "KubeProxyIncorrectIPVersion", errMsg)
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
