package localnetv1

import (
	"net"
)

// AddAddress adds an address to this endpoint, returning the parsed IP. `ìp` will be nil if it couldn't be parsed.
func (ep *Endpoint) AddAddress(s string) (ip net.IP) {
	if ep.IPs == nil {
		ep.IPs = &IPSet{}
	}

	return ep.IPs.Add(s)
}
