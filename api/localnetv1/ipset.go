/*
Copyright 2021 The Kubernetes Authors.

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

package localnetv1

import (
	"net"
	"sort"
)

func NewIPSet(ips ...string) (set *IPSet) {
	set = &IPSet{}
	set.AddAll(ips)
	return
}

func (set *IPSet) IsEmpty() bool {
	return len(set.V4) == 0 && len(set.V6) == 0
}

func (set *IPSet) First() string {
	if len(set.V4) != 0 {
		return set.V4[0]
	}
	if len(set.V6) != 0 {
		return set.V6[0]
	}
	return ""
}

// Add adds an address to this set, returning the parsed IP. `ìp` will be nil if it couldn't be parsed.
func (set *IPSet) Add(s string) (ip net.IP) {
	ip = net.ParseIP(s)
	if ip == nil {
		return
	}

	if ip.To4() == nil {
		insertString(&set.V6, s)
	} else {
		insertString(&set.V4, s)
	}

	return
}

func (set *IPSet) AddAll(ips []string) {
	for _, ip := range ips {
		set.Add(ip)
	}
}

func (set *IPSet) AddSet(set2 *IPSet) {
	if set2 == nil {
		return
	}

	for _, ip := range set2.V4 {
		insertString(&set.V4, ip)
	}
	for _, ip := range set2.V6 {
		insertString(&set.V6, ip)
	}
}

func insertString(a *[]string, s string) {
	idx := sort.SearchStrings(*a, s)

	if idx != len(*a) && (*a)[idx] == s {
		// already there
		return
	}

	// insert
	(*a) = append(*a, "")
	copy((*a)[idx+1:], (*a)[idx:])
	(*a)[idx] = s
	return
}

func (set *IPSet) All() []string {
	if set == nil {
		return nil
	}

	all := make([]string, 0, len(set.V4)+len(set.V6))
	all = append(all, set.V4...)
	all = append(all, set.V6...)
	return all
}

func (from *IPSet) Diff(to *IPSet) (added, removed *IPSet) {
	added = &IPSet{}
	removed = &IPSet{}

	added.V4, removed.V4 = diffStrings(from.V4, to.V4)
	added.V6, removed.V6 = diffStrings(from.V6, to.V6)
	return
}

// compareSlices returns the difference between the two slices
func compareSlices(from, to []string) (diff []string) {
	for _, s1 := range from {
		found := false
		for _, s2 := range to {
			if s1 == s2 {
				found = true
				break
			}
		}

		if !found {
			diff = append(diff, s1)
		}
	}
	return
}

// diffStrings will compare two slices and will return the strings
// added and removed at the same time.
func diffStrings(from, to []string) (added, removed []string) {
	added = compareSlices(to, from)
	removed = compareSlices(from, to)
	return added, removed

}
