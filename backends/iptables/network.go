package iptables

import (
	"net"
)

// NetworkInterfacer defines an interface for several net library functions. Production
// code will forward to net library functions, and unit tests will override the methods
// for testing purposes.
type NetworkInterfacer interface {
	Addrs(intf *net.Interface) ([]net.Addr, error)
	Interfaces() ([]net.Interface, error)
}

// RealNetwork implements the NetworkInterfacer interface for production code, just
// wrapping the underlying net library function calls.
//type RealNetwork struct{}

// Addrs wraps net.Interface.Addrs(), it's a part of NetworkInterfacer interface.
//func (RealNetwork) Addrs(intf *net.Interface) ([]net.Addr, error) {
//	return intf.Addrs()
//}

// Interfaces wraps net.Interfaces(), it's a part of NetworkInterfacer interface.
//func (RealNetwork) Interfaces() ([]net.Interface, error) {
//	return net.Interfaces()
//}

//var _ NetworkInterfacer = &RealNetwork{}
