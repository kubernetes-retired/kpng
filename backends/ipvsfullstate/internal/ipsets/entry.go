package ipsets

import (
	"fmt"
	"k8s.io/klog/v2"
	"net"
	"sigs.k8s.io/kpng/api/localv1"
	"strconv"
	"strings"
)

// Entry represents an ipset entry.
type Entry struct {
	// IP is the entry's IP.  The IP address protocol corresponds to the HashFamily of Set.
	// All entries' IP addresses in the same ip set has same the protocol, IPv4 or IPv6.
	IP string
	// Port is the entry's Port.
	Port int
	// Protocol is the entry's Protocol.  The protocols of entries in the same ip set are all
	// the same.  The accepted protocols are TCP, UDP and SCTP.
	Protocol localv1.Protocol
	// Net is the entry's IP network address.  Network address with zero prefix size can NOT
	// be stored.
	Net string
	// IP2 is the entry's second IP.  IP2 may not be empty for `hash:ip,port,ip` type ip set.
	IP2 string
	// SetType is the type of ipset where the entry exists.
	SetType SetType

	set *Set
}

// Validate checks if a given ipset entry is valid or not.  The set parameter is the ipset that entry belongs to.
func (e *Entry) Validate(set *Set) bool {
	if e.Port < 0 {
		klog.Errorf("Entry %v port number %d should be >=0 for ipset %v", e, e.Port, set)
		return false
	}
	switch e.SetType {
	case HashIPPort:
		//check if IP and Protocol of Entry is valid.
		if valid := e.checkIPAndProtocol(set); !valid {
			return false
		}
	case HashIPPortIP:
		//check if IP and Protocol of Entry is valid.
		if valid := e.checkIPAndProtocol(set); !valid {
			return false
		}

		// IP2 can not be empty for `hash:ip,port,ip` type ip set
		if net.ParseIP(e.IP2) == nil {
			klog.Errorf("Error parsing entry %v second ip address %v for ipset %v", e, e.IP2, set)
			return false
		}
	case HashIPPortNet:
		//check if IP and Protocol of Entry is valid.
		if valid := e.checkIPAndProtocol(set); !valid {
			return false
		}

		// Net can not be empty for `hash:ip,port,net` type ip set
		if _, ipNet, err := net.ParseCIDR(e.Net); ipNet == nil {
			klog.Errorf("Error parsing entry %v ip net %v for ipset %v, error: %v", e, e.Net, set, err)
			return false
		}
	case BitmapPort:
		// check if port number satisfies its ipset's requirement of port range
		if set == nil {
			klog.Errorf("Unable to reference ip set where the entry %v exists", e)
			return false
		}
		begin, end, err := parsePortRange(set.PortRange)
		if err != nil {
			klog.Errorf("Failed to parse set %v port range %s for ipset %v, error: %v", set, set.PortRange, set, err)
			return false
		}
		if e.Port < begin || e.Port > end {
			klog.Errorf("Entry %v port number %d is not in the port range %s of its ipset %v", e, e.Port, set.PortRange, set)
			return false
		}
	}

	return true
}

// String returns the string format for ipset entry.
func (e *Entry) String() string {

	// convert the protocol from upstream KPNG server type to string type required by IPSets.
	protocol := strings.ToLower(e.Protocol.String())

	switch e.SetType {
	case HashIPPort:
		// Entry{192.168.1.1, udp, 53} -> 192.168.1.1,udp:53
		// Entry{192.168.1.2, tcp, 8080} -> 192.168.1.2,tcp:8080
		return fmt.Sprintf("%s,%s:%s", e.IP, protocol, strconv.Itoa(e.Port))
	case HashIPPortIP:
		// Entry{192.168.1.1, udp, 53, 10.0.0.1} -> 192.168.1.1,udp:53,10.0.0.1
		// Entry{192.168.1.2, tcp, 8080, 192.168.1.2} -> 192.168.1.2,tcp:8080,192.168.1.2
		return fmt.Sprintf("%s,%s:%s,%s", e.IP, protocol, strconv.Itoa(e.Port), e.IP2)
	case HashIPPortNet:
		// Entry{192.168.1.2, udp, 80, 10.0.1.0/24} -> 192.168.1.2,udp:80,10.0.1.0/24
		// Entry{192.168.2,25, tcp, 8080, 10.1.0.0/16} -> 192.168.2,25,tcp:8080,10.1.0.0/16
		return fmt.Sprintf("%s,%s:%s,%s", e.IP, protocol, strconv.Itoa(e.Port), e.Net)
	case BitmapPort:
		// Entry{53} -> 53
		// Entry{8080} -> 8080
		return strconv.Itoa(e.Port)
	}
	return ""
}

// checkIPAndProtocol checks if IP and Protocol of Entry is valid.
func (e *Entry) checkIPAndProtocol(set *Set) bool {
	// set default protocol to tcp if empty
	if !validateProtocol(e.Protocol) {
		return false
	}

	if net.ParseIP(e.IP) == nil {
		klog.Errorf("Error parsing entry %v ip address %v for ipset %v", e, e.IP, set)
		return false
	}

	return true
}

// checks if port range is valid. The begin port number is not necessarily less than
// end port number - ipset util can accept it.  It means both 1-100 and 100-1 are valid.
func validatePortRange(portRange string) bool {
	strs := strings.Split(portRange, "-")
	if len(strs) != 2 {
		klog.Errorf("port range should be in the format of `a-b`")
		return false
	}
	for i := range strs {
		num, err := strconv.Atoi(strs[i])
		if err != nil {
			klog.Errorf("Failed to parse %s, error: %v", strs[i], err)
			return false
		}
		if num < 0 {
			klog.Errorf("port number %d should be >=0", num)
			return false
		}
	}
	return true
}

// checks if given hash family is supported in ipset
func validateHashFamily(family string) bool {
	if family == ProtocolFamilyIPV4 || family == ProtocolFamilyIPV6 {
		return true
	}
	klog.Errorf("Currently supported ip set hash families are: [%s, %s], %s is not supported", ProtocolFamilyIPV4, ProtocolFamilyIPV6, family)
	return false
}

// checks if given protocol is supported in entry
func validateProtocol(protocol localv1.Protocol) bool {
	_protocol := strings.ToLower(protocol.String())
	if _protocol == ProtocolTCP || _protocol == ProtocolUDP || _protocol == ProtocolSCTP {
		return true
	}
	klog.Errorf("Invalid entry's protocol: %s, supported protocols are [%s, %s, %s]", protocol, ProtocolTCP, ProtocolUDP, ProtocolSCTP)
	return false
}

// parsePortRange parse the beginning and end port from a raw string(format: a-b).  beginPort <= endPort
// in the return value.
func parsePortRange(portRange string) (beginPort int, endPort int, err error) {
	if len(portRange) == 0 {
		portRange = DefaultPortRange
	}

	strs := strings.Split(portRange, "-")
	if len(strs) != 2 {
		// port number -1 indicates invalid
		return -1, -1, fmt.Errorf("port range should be in the format of `a-b`")
	}
	for i := range strs {
		num, err := strconv.Atoi(strs[i])
		if err != nil {
			// port number -1 indicates invalid
			return -1, -1, err
		}
		if num < 0 {
			// port number -1 indicates invalid
			return -1, -1, fmt.Errorf("port number %d should be >=0", num)
		}
		if i == 0 {
			beginPort = num
			continue
		}
		endPort = num
		// switch when first port number > second port number
		if beginPort > endPort {
			endPort = beginPort
			beginPort = num
		}
	}
	return beginPort, endPort, nil
}
