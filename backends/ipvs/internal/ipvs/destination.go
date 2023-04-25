package ipvs

import (
	IPVSLib "github.com/google/seesaw/ipvs"
	"net"
	"strconv"
)

// Destination represents a virtual server's destination. This structure should
// not hold any static information (e.g. weight) to save memory.
// The data types of this structure should match the data types given by the KPNG
// server to avoid type casting and parsing, saving cpu.
type Destination struct {
	virtualServer *VirtualServer
	IP            string
	Port          int32
}

// Key returns a combination of ip, port and protocol of the virtual server.
func (d *Destination) Key() string {
	// format: tcp://10.96.1.10:80/10.1.1.2:8000
	return d.virtualServer.Key() + "/" + d.IPPort()
}

// Equal compares two destinations and their virtual servers, is used for diffstore equality assertion.
func (d *Destination) Equal(o *Destination) bool {
	if d.IP == o.IP && d.Port == o.Port {
		if d.virtualServer != nil && o.virtualServer != nil {
			// No need to call Equal() on virtual server as we don't destination only needs
			// to be reprogrammed in case of change of IP, Port or Protocol of virtual server.
			return d.virtualServer.Key() == o.virtualServer.Key()
		}
		return true
	}
	return false
}

// IPPort return combination of IP and Port for the destination.
func (d *Destination) IPPort() string {
	return d.IP + ":" + strconv.Itoa(int(d.Port))
}

// asIPVSLibDestination adds all static parameters, does all parsing and type casting
// and adapts the structure for destination manipulation in kernel.
func (d *Destination) asIPVSLibDestination(weight int32) IPVSLib.Destination {
	return IPVSLib.Destination{
		Address: net.ParseIP(d.IP),
		Port:    uint16(d.Port),
		Weight:  weight,
	}
}
