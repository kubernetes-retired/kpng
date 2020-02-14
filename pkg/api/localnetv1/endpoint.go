package localnetv1

import (
	"net"
	"sort"
)

// AddAddress adds an address to this endpoint, returning the parsed IP. `ìp` will be nil if it couldn't be parsed.
func (ep Endpoint) AddAddress(s string) (ip net.IP) {
	ip = net.ParseIP(s)
	if ip == nil {
		return
	}

	if ip.To4() == nil {
		ep.IPsV6 = insertString(ep.IPsV6, s)
	} else {
		ep.IPsV4 = insertString(ep.IPsV4, s)
	}

	return
}

func insertString(a []string, s string) []string {
	idx := sort.SearchStrings(a, s)

	if idx != len(a) && a[idx] == s {
		// already there
		return a
	}

	// insert
	a = append(a, "")
	copy(a[idx+1:], a[idx:])
	a[idx] = s
	return a
}
