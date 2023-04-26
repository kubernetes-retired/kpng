/*
Copyright 2023 The Kubernetes Authors.

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

package ipsets

import (
	"k8s.io/klog/v2"
)

// Interface is an injectable interface for running ipset commands.  Implementations must be goroutine-safe.
type Interface interface {
	// FlushSet deletes all entries from a named set.
	FlushSet(set string) error
	// DestroySet deletes a named set.
	DestroySet(set string) error
	// DestroyAllSets deletes all sets.
	DestroyAllSets() error
	// CreateSet creates a new set.  It will ignore error when the set already exists if ignoreExistErr=true.
	CreateSet(set *IPSet, ignoreExistErr bool) error
	// AddEntry adds a new entry to the named set.  It will ignore error when the entry already exists if ignoreExistErr=true.
	AddEntry(entry string, set *IPSet, ignoreExistErr bool) error
	// DelEntry deletes one entry from the named set
	DelEntry(entry string, set string) error
	// Test test if an entry exists in the named set
	TestEntry(entry string, set string) (bool, error)
	// ListEntries lists all the entries from a named set
	ListEntries(set string) ([]string, error)
	// ListSets list all set names from kernel
	ListSets() ([]string, error)
	// GetVersion returns the "X.Y" version string for ipset.
	GetVersion() (string, error)
}

// IPSetCmd represents the ipset util. We use ipset command for ipset execute.
const IPSetCmd = "ipset"

// EntryMemberPattern is the regular expression pattern of ipset member list.
// The raw output of ipset command `ipset list {set}` is similar to,
// Name: foobar
// Type: hash:ip,port
// Revision: 2
// Header: family inet hashsize 1024 maxelem 65536
// Size in memory: 16592
// References: 0
// Members:
// 192.168.1.2,tcp:8080
// 192.168.1.1,udp:53
var EntryMemberPattern = "(?m)^(.*\n)*Members:\n"

// VersionPattern is the regular expression pattern of ipset version string.
// ipset version output is similar to "v6.10".
var VersionPattern = "v[0-9]+\\.[0-9]+"

// IPSet implements an Interface to a set.
type IPSet struct {
	// Name is the set name.
	Name string
	// SetType specifies the ipset type.
	SetType SetType
	// HashFamily specifies the protocol family of the IP addresses to be stored in the set.
	// The default is inet, i.e IPv4.  If users want to use IPv6, they should specify inet6.
	HashFamily ProtocolFamily
	// HashSize specifies the hash table size of ipset.
	HashSize int
	// MaxElem specifies the max element number of ipset.
	MaxElem int
	// PortRange specifies the port range of bitmap:port type ipset.
	PortRange string
	// comment message for ipset
	Comment string
}

// Validate checks if a given ipset is valid or not.
func (set *IPSet) Validate() bool {
	// Check if protocol is valid for `HashIPPort`, `HashIPPortIP` and `HashIPPortNet` type set.
	if set.SetType == HashIPPort || set.SetType == HashIPPortIP || set.SetType == HashIPPortNet {
		if valid := validateHashFamily(set.HashFamily); !valid {
			return false
		}
	}
	// check set type
	if valid := validateIPSetType(set.SetType); !valid {
		return false
	}
	// check port range for bitmap type set
	if set.SetType == BitmapPort {
		if valid := validatePortRange(set.PortRange); !valid {
			return false
		}
	}
	// check hash size value of ipset
	if set.HashSize <= 0 {
		klog.Errorf("Invalid hashsize value %d, should be >0", set.HashSize)
		return false
	}
	// check max elem value of ipset
	if set.MaxElem <= 0 {
		klog.Errorf("Invalid maxelem value %d, should be >0", set.MaxElem)
		return false
	}

	return true
}

// setIPSetDefaults sets some IPSet fields if not present to their default values.
func (set *IPSet) setIPSetDefaults() {
	// Setting default values if not present
	if set.HashSize == 0 {
		set.HashSize = 1024
	}
	if set.MaxElem == 0 {
		set.MaxElem = 65536
	}
	// Default protocol is IPv4
	if set.HashFamily == "" {
		set.HashFamily = ProtocolFamilyIPv4
	}
	// Default ipset type is "hash:ip,port"
	if len(set.SetType) == 0 {
		set.SetType = HashIPPort
	}
	if len(set.PortRange) == 0 {
		set.PortRange = DefaultPortRange
	}
}

// checks if the given ipset type is valid.
func validateIPSetType(set SetType) bool {
	for _, valid := range ValidIPSetTypes {
		if set == valid {
			return true
		}
	}
	klog.Errorf("Currently supported ipset types are: %v, %s is not supported", ValidIPSetTypes, set)
	return false
}
