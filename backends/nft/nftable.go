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

package nft

import (
	"sigs.k8s.io/kpng/client/diffstore2"
)

var (
	table4 = newNftable("ip", "k8s_svc")
	table6 = newNftable("ip6", "k8s_svc6")

	allTables = []*nftable{table4, table6}
)

type Leaf = diffstore2.BufferLeaf
type Item = diffstore2.Item[string, *Leaf]
type Store = diffstore2.Store[string, *Leaf]

// newNfttable returns an nftable, which defines a standard nft table.
// nft tables include:
// - a family of packets (ip, ip6, arp, ...)
// - a list of many "chains"
//   - each "chain" contains many "rules"
//
// For example, the following table would be created by running individual commands
// to create the "filter0" table, add the "filter0_chain0" chain to it, and then to
// add the "type filter hook input..." rule.
//
// table ip filter0 {
//        chain filter0_chain0 {
//                type filter hook input priority filter; policy accept;
//        }
//
// After creating the above table, we could then add more chains, like so:
//
//        chain udp_packets {
//        }
//
//        chain incoming_traffic {
//                type filter hook input priority filter; policy accept;
//                ip protocol vmap { udp : jump udp_packets }
//        }
// }
type nftable struct {
	Family string
	Name   string

	// The Chain and Map items of a table are the "dynanmic" parts that change as new
	// service and endpoint data is sent to a KPNG backend (i.e. from the KPNG brain).
	// We thus model these as DiffStore objects, so that we can detect wether there are any
	// material changes which might require us to rewrite the NFT rules to the linux kernel.
	Chains *Store
	Maps   *Store
}

func newNftable(family, name string) *nftable {
	return &nftable{
		Family: family,
		Name:   name,
		// As mentioned above, a NFT table can have chains and maps to manage routing rules.
		// We model these as "diffstore" objects, rather then raw maps, to track the differences
		// as mentioned above.
		Chains: diffstore2.NewBufferStore[string](),
		Maps:   diffstore2.NewBufferStore[string](),
	}
}

// Reset resets the individual diffstores' for the Chain and Map items of this table.
// The Chain and Maps of an Nftable are the "things" which change throughout the process
// of the KPNG nft backend, as new endpoints and services are discovered from the KPNG brain.
func (n *nftable) Reset() {
	n.Chains.Reset()
	n.Maps.Reset()
}

func (n *nftable) RunDeferred() {
	n.Chains.RunDeferred()
	n.Maps.RunDeferred()
}

func (n *nftable) FinalizeDiffHashes() {
	n.Chains.FinalizeDiffHashes()
	n.Maps.FinalizeDiffHashes()
}

type KindStore struct {
	Kind  string
	Store *Store
}

func (n *nftable) KindStores() []KindStore {
	return []KindStore{
		{"map", n.Maps},
		{"chain", n.Chains},
	}
}

func (n *nftable) Changed() bool {
	return n.Chains.HasChanges() || n.Maps.HasChanges()
}
