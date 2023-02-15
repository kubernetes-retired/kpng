package ipvs

import (
	IPVSLib "github.com/google/seesaw/ipvs"
	"net"
	"sigs.k8s.io/kpng/api/localv1"
	"strconv"
	"syscall"
)

// VirtualServer represents a virtual server. This structure should not
// hold any static information (eg. scheduling method) to save memory.
// The data types of this structure should match the data types given by
// the KPNG server to avoid type casting and parsing, saving cpu.
type VirtualServer struct {
	IP       string
	Port     int32
	Protocol localv1.Protocol
	Timeout  int32
}

// Key returns a combination of IP, Port and Protocol of the virtual server.
func (vs VirtualServer) Key() string {
	// format: tcp://10.96.1.10:80
	return vs.Protocol.String() + "://" + vs.IPPort()
}

// Equal compares two virtual servers, is used for diffstore equality assertion.
func (vs VirtualServer) Equal(o *VirtualServer) bool {
	return vs.IP == o.IP &&
		vs.Port == o.Port &&
		vs.Protocol == o.Protocol &&
		vs.Timeout == o.Timeout
}

// IPPort return combination of IP and port for the server
func (vs VirtualServer) IPPort() string {
	return vs.IP + ":" + strconv.Itoa(int(vs.Port))
}

// asIPVSLibService adds all static parameters, does all parsing and type casting
// and adapts the structure for virtual server manipulation in kernel.
func (vs VirtualServer) asIPVSLibService(schedulingMethod string) IPVSLib.Service {
	lb := IPVSLib.Service{
		Address:   net.ParseIP(vs.IP),
		Port:      uint16(vs.Port),
		Protocol:  getProtocolForIPVS(vs.Protocol),
		Scheduler: schedulingMethod,
	}
	if vs.Timeout > 0 {
		lb.Flags |= IPVSLib.SFPersistent
		lb.Timeout = uint32(vs.Timeout)
	}
	return lb
}

// converts localv1.Protocol protocol to IPVSLib.IPProto.
func getProtocolForIPVS(protocol localv1.Protocol) IPVSLib.IPProto {
	switch protocol {
	case localv1.Protocol_TCP:
		return syscall.IPPROTO_TCP
	case localv1.Protocol_UDP:
		return syscall.IPPROTO_UDP
	case localv1.Protocol_SCTP:
		return syscall.IPPROTO_SCTP
	default:
		// defaulting protocol to TCP
		return syscall.IPPROTO_TCP
	}
}
