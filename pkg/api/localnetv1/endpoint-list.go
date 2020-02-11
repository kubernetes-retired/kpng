package localnetv1

import (
	"net"
	"sort"
)

func (l *EndpointList) Add(s string) {
	ip := net.ParseIP(s)
	if ip == nil {
		l.Hostnames = insertString(l.Hostnames, s)
	} else if ip.To4() == nil {
		l.IPsV6 = insertString(l.IPsV6, s)
	} else {
		l.IPsV4 = insertString(l.IPsV4, s)
	}
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

func (l *EndpointList) ResetSets() {
	if l.Hostnames != nil {
		l.Hostnames = l.Hostnames[:0]
	}
	if l.IPsV4 != nil {
		l.IPsV4 = l.IPsV4[:0]
	}
	if l.IPsV6 != nil {
		l.IPsV6 = l.IPsV6[:0]
	}
}
