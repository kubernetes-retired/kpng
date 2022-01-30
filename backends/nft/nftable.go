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
	"bytes"
	"fmt"
	"github.com/OneOfOne/xxhash"
	"github.com/google/btree"
	"io"
	"sigs.k8s.io/kpng/client/diffstore2"
)

var (
	// nftTableManager = newNFTManager()
	chainTypes = map[string]bool{"chain": true, "map": true}
)

type NFTManager struct {
	allTables []*nftable
}

func newNFTManager() *NFTManager {
	n := &NFTManager{}
	n.allTables = []*nftable{
		newNftable("ip", "k8s_svc"),
		newNftable("ip6", "k8s_svc6"),
	}
	return n
}

// GetV4Table returns a singleton instance of the global DiffState table for ipv4
func (n *NFTManager) GetV4Table() *nftable {
	return n.allTables[0]
}

// GetV6Table returns a singleton instance of the global DiffState table for ipv6
func (n *NFTManager) GetV6Table() *nftable {
	return n.allTables[1]
}

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

	// this allows this datastructure to be reused as a btree
	data *btree.BTree
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
func (n *nftable) ResetNFTForChainMaps() {
	n.Chains.Reset()
	n.Maps.Reset()
}

func (n *nftable) RunDeferredForChainMaps() {
	n.Chains.RunDeferred()
	n.Maps.RunDeferred()
}

func (n *nftable) FinalizeDiffHashesForChainMaps() {
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

func (n *nftable) isDirtyChainOrMap() bool {
	return n.Chains.HasChanges() || n.Maps.HasChanges()
}

//

// chainBuffer is our underlying storage medium for all the chains in a table.
// We use hashes to rapidly check wether chains need to be rewritten.
// the buffer
type chainBuffer struct {
	kind         string
	name         string
	previousHash uint64
	currentHash  *xxhash.XXHash64
	buffer       *bytes.Buffer
	lenMA        int
	deferred     []func(*chainBuffer)
}

var (
	_ btree.Item    = &chainBuffer{}
	_ io.ReadWriter = &chainBuffer{}
)

// Less implements the interface for the btree which we use to store all the chains in this table.
// See the Item interface for our underlying BTree implementation for details.
func (c *chainBuffer) Less(i btree.Item) bool {
	return c.name < i.(*chainBuffer).name
}

func (c *chainBuffer) Read(b []byte) (int, error) {
	return c.buffer.Read(b)
}

func (c *chainBuffer) Write(b []byte) (int, error) {
	if c.currentHash == nil {
		c.currentHash = xxhash.New64()
	}
	c.currentHash.Write(b)
	return c.buffer.Write(b)
}

func (c *chainBuffer) Writeln() (n int, err error) {
	return c.Write([]byte{'\n'})
}

func (c *chainBuffer) WriteString(s string) (n int, err error) {
	start := c.buffer.Len()
	n, err = c.buffer.WriteString(s)

	if c.currentHash == nil {
		c.currentHash = xxhash.New64()
	}
	c.currentHash.Write(c.buffer.Bytes()[start:])

	return n, err
}

func (c *chainBuffer) Len() int {
	return c.buffer.Len()
}

// Changed uses the hash computation in this chain buffer to determine wether or not
// any recent changes have occured on this chain.
func (c *chainBuffer) Changed() bool {
	if c.currentHash == nil {
		return c.previousHash != 0
	}
	return c.currentHash.Sum64() != c.previousHash
}

// Defer runs adds a function to the queue of tasks which we will run on the chain Buffer.
func (c *chainBuffer) Defer(deferred func(*chainBuffer)) {
	c.deferred = append(c.deferred, deferred)
}

// RunDeffered runs all the deferred operations for this chainBuffer.
func (c *chainBuffer) RunDeferred() {
	for _, deferredOperation := range c.deferred {
		deferredOperation(c)
	}
}

func (c *chainBuffer) Created() bool {
	return c.previousHash == 0 && c.currentHash != nil
}

func (set *nftable) ResetTree() {
	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*chainBuffer)

		cb.deferred = cb.deferred[:0]

		// compute buffer len moving average
		if cb.lenMA == 0 {
			cb.lenMA = cb.buffer.Len()
		} else {
			cb.lenMA = (4*cb.lenMA + cb.buffer.Len()) / 5
		}
		// expect len+20%
		expCap := cb.lenMA * 120 / 100

		if cb.buffer.Cap() <= expCap {
			cb.buffer.Reset()
		} else {
			cb.buffer = bytes.NewBuffer(make([]byte, 0, expCap))
		}

		if cb.currentHash == nil {
			// no writes -> empty
			cb.previousHash = 0
		} else {
			cb.previousHash = cb.currentHash.Sum64()
			cb.currentHash = nil
		}
		return true
	})
}

// ReplaceOrInsert creates a new chainBuffer which can be used to add new chains.
// A typical NFT chain might start off empty...
// nft 'add chain filter0 filter0_chain0 { }
// Once a chain is created we can add rules to it.
func (set *nftable) ReplaceOrInsert(kind, name string) *chainBuffer {
	i := set.data.Get(&chainBuffer{name: name})

	// we didn't see this chain - so we'll make a new one...
	if i == nil {
		if _, ok := chainTypes[kind]; !ok {
			chainError := fmt.Errorf("can't create chain buffer w/o with kind %v", kind)
			panic(chainError)
		}

		i = &chainBuffer{
			kind:     kind,
			name:     name,
			buffer:   new(bytes.Buffer),
			deferred: make([]func(*chainBuffer), 0, 1),
		}
		set.data.ReplaceOrInsert(i)
	}

	cb := i.(*chainBuffer)

	if kind != "" && kind != cb.kind {
		panic("wrong kind for " + name + ": " + kind + " (got " + cb.kind + ")")
	}

	return cb
}

// ListChains returns a list of all the chain names in this nftable.
func (set *nftable) ListChains() (chains []string) {
	chains = make([]string, 0, set.data.Len())

	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*chainBuffer)
		if cb.currentHash != nil {
			chains = append(chains, cb.name)
		}
		return true
	})

	return
}

func (set *nftable) Deleted() (chains []*chainBuffer) {
	chains = make([]*chainBuffer, 0)

	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*chainBuffer)
		if cb.previousHash != 0 && cb.currentHash == nil {
			chains = append(chains, cb)
		}
		return true
	})

	return
}

func (set *nftable) ChangedTree() (changed bool) {
	changed = false

	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*chainBuffer)
		if cb.Changed() {
			changed = true
		}
		return !changed
	})

	return
}

// RunDeferred runs all of the deferred tasks on this nftable in ascending order, of each chain.
func (set *nftable) RunDeferred() {
	set.data.Ascend(
		func(i btree.Item) bool {
			// Run all of the deferred inner tasks of this chain.
			i.(*chainBuffer).RunDeferred()
			return true
		})
}
