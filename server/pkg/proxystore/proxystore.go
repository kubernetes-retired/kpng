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

package proxystore

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/cespare/xxhash"
	"github.com/google/btree"
	"google.golang.org/protobuf/proto"
	"k8s.io/klog"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

type Store struct {
	sync.RWMutex
	c      *sync.Cond
	rev    uint64
	closed bool
	tree   *btree.BTree

	// set sync info
	sync map[Set]bool
}

type Set = localnetv1.Set

const (
	Services  = localnetv1.Set_GlobalServiceInfos
	Endpoints = localnetv1.Set_GlobalEndpointInfos
	Nodes     = localnetv1.Set_GlobalNodeInfos
)

var AllSets = []Set{Services, Endpoints, Nodes}

type Hashed interface {
	GetHash() uint64
}

func New() *Store {
	return &Store{
		c:    sync.NewCond(&sync.Mutex{}),
		tree: btree.New(2),
		sync: map[Set]bool{},
	}
}

func (s *Store) hashOf(m proto.Message) (h uint64) {
	message, err := proto.Marshal(m)
	if err != nil {
		panic(err) // should not happen
	}
	return xxhash.Sum64(message)
}

func (s *Store) Close() {
	s.c.L.Lock()
	s.closed = true
	s.c.Broadcast()
	s.c.L.Unlock()
}

func (s *Store) Update(update func(tx *Tx)) {
	s.Lock()
	defer s.Unlock()

	tx := &Tx{s: s}
	update(tx)

	if tx.changes == 0 {
		return // nothing changed
	}

	// TODO check if the update really updated something
	s.c.L.Lock()
	s.rev++
	s.c.Broadcast()
	s.c.L.Unlock()

	if log := klog.V(3); log {
		log.Info("store updated to rev ", s.rev, " with ", s.tree.Len(), " entries")
		if log := klog.V(4); log {
			s.tree.Ascend(func(i btree.Item) bool {
				kv := i.(*KV)
				log.Info("- entry: ", kv.Sync, "/", kv.Set, ": ", kv.Namespace, "/", kv.Name, "/", kv.Source, "/", kv.Key)
				return true
			})
		}
	}
}

func (s *Store) View(afterRev uint64, view func(tx *Tx)) (rev uint64, closed bool) {
	s.c.L.Lock()
	for s.rev <= afterRev && !s.closed {
		s.c.Wait()
	}
	s.c.L.Unlock()

	s.RLock()
	defer s.RUnlock()

	if s.closed {
		return 0, s.closed
	}

	view(&Tx{s: s, ro: true})

	return s.rev, s.closed
}

type Tx struct {
	s       *Store
	ro      bool
	changes uint
}

func (tx *Tx) roPanic() {
	if tx.ro {
		panic("read-only!")
	}
}

// Each iterate over each item in the given set, stopping if the callback returns false
func (tx *Tx) Each(set Set, callback func(*KV) bool) {
	tx.s.tree.AscendGreaterOrEqual(&KV{Set: set}, func(i btree.Item) bool {
		kv := i.(*KV)

		if kv.Set != set {
			return false
		}

		return callback(kv)
	})
}

// Reset clears the store data
func (tx *Tx) Reset() {
	tx.roPanic()

	if tx.s.tree.Len() != 0 {
		tx.s.tree.Clear(false)
		tx.changes++
	}

	for set, isSync := range tx.s.sync {
		if isSync {
			tx.s.sync[set] = false
			tx.changes++
		}
	}
}

func (tx *Tx) set(kv *KV) {
	tx.roPanic()
	prev := tx.s.tree.Get(kv)

	if prev != nil && prev.(*KV).Value.GetHash() == kv.Value.GetHash() {
		return // not changed
	}

	tx.s.tree.ReplaceOrInsert(kv)
	tx.changes++
}

func (tx *Tx) del(kv *KV) {
	tx.roPanic()
	i := tx.s.tree.Delete(kv)
	if i != nil {
		tx.changes++
	}
}

func (tx *Tx) SetRaw(set Set, path string, value Hashed) {
	kv := &KV{}
	kv.Set = set
	kv.SetPath(path)

	kv.Value = value

	switch v := value.(type) {
	case *localnetv1.NodeInfo:
		kv.Node = v
		tx.set(kv)

	case *localnetv1.ServiceInfo:
		kv.Service = v
		tx.set(kv)

	case *localnetv1.EndpointInfo:
		kv.Endpoint = v
		tx.set(kv)

	default:
		panic(fmt.Errorf("unknown value type: %t", v))
	}
}

func (tx *Tx) DelRaw(set Set, path string) {
	kv := &KV{}
	kv.Set = set
	kv.SetPath(path)

	tx.del(kv)
}

// sync funcs
func (tx *Tx) AllSynced() bool {
	for _, set := range []Set{Services, Endpoints, Nodes} {
		if !tx.IsSynced(set) {
			return false
		}
	}
	return true
}
func (tx *Tx) IsSynced(set Set) bool {
	return tx.s.sync[set]
}
func (tx *Tx) SetSync(set Set) {
	tx.roPanic()

	if !tx.s.sync[set] {
		tx.s.sync[set] = true
		tx.changes++
	}
}

// Services funcs

func (tx *Tx) SetService(s *localnetv1.Service, topologyKeys []string) {
	si := &localnetv1.ServiceInfo{
		Service:      s,
		TopologyKeys: topologyKeys,
		Hash: tx.s.hashOf(&localnetv1.ServiceInfo{
			Service:      s,
			TopologyKeys: topologyKeys,
		}),
	}

	tx.set(&KV{
		Set:       Services,
		Namespace: s.Namespace,
		Name:      s.Name,
		Value:     si,
		Service:   si,
	})
}

func (tx *Tx) DelService(namespace, name string) {
	tx.del(&KV{
		Set:       Services,
		Namespace: namespace,
		Name:      name,
	})
}

// Endpoints funcs

func (tx *Tx) EachEndpointOfService(namespace, serviceName string, callback func(*localnetv1.EndpointInfo)) {
	tx.s.tree.AscendGreaterOrEqual(&KV{
		Set:       Endpoints,
		Namespace: namespace,
		Name:      serviceName,
	}, func(i btree.Item) bool {
		kv := i.(*KV)

		if kv.Set != Endpoints || kv.Namespace != namespace || kv.Name != serviceName {
			return false
		}

		callback(kv.Endpoint)

		return true
	})
}

// SetEndpointsOfSource replaces ALL endpoints of a single source (add new, update existing, delete removed)
func (tx *Tx) SetEndpointsOfSource(namespace, sourceName string, eis []*localnetv1.EndpointInfo) {
	tx.roPanic()

	seen := map[uint64]bool{}

	for _, ei := range eis {
		if ei.Namespace != namespace {
			panic("inconsistent namespace: " + namespace + " != " + ei.Namespace)
		}
		if ei.SourceName != sourceName {
			panic("inconsistent source: " + sourceName + " != " + ei.SourceName)
		}

		ei.Hash = tx.s.hashOf(&localnetv1.EndpointInfo{
			Endpoint:   ei.Endpoint,
			Conditions: ei.Conditions,
			Topology:   ei.Topology,
		})
		seen[ei.Hash] = true
	}

	// to delete unseen endpoints
	toDel := make([]*KV, 0)

	tx.s.tree.AscendGreaterOrEqual(&KV{
		Set:       Endpoints,
		Namespace: namespace,
		Source:    sourceName,
	}, func(i btree.Item) bool {
		kv := i.(*KV)
		if kv.Set != Endpoints || kv.Namespace != namespace || kv.Source != sourceName {
			return false
		}

		if !seen[kv.Endpoint.Hash] {
			ei := kv.Endpoint
			toDel = append(toDel,
				kv,
				&KV{ // also remove the reference in the service
					Set:       Endpoints,
					Namespace: ei.Namespace,
					Name:      ei.ServiceName,
					Source:    ei.SourceName,
					Key:       kv.Key,
				})
		}

		return true
	})

	for _, toDel := range toDel {
		tx.del(toDel)
	}

	// add/update known endpoints
	for _, ei := range eis {
		key := strconv.FormatUint(ei.Hash, 16)

		kv := &KV{
			Set:       Endpoints,
			Namespace: ei.Namespace,
			Name:      ei.ServiceName,
			Source:    ei.SourceName,
			Key:       key,
			Value:     ei,
			Endpoint:  ei,
		}

		if tx.s.tree.Has(kv) {
			continue
		}

		tx.set(kv)

		// also index by source only
		tx.set(&KV{
			Set:       Endpoints,
			Namespace: ei.Namespace,
			Source:    ei.SourceName,
			Key:       key,
			Value:     ei,
			Endpoint:  ei,
		})
	}
}

func (tx *Tx) DelEndpointsOfSource(namespace, sourceName string) {
	tx.roPanic()

	toDel := make([]*KV, 0)

	tx.s.tree.AscendGreaterOrEqual(&KV{
		Set:       Endpoints,
		Namespace: namespace,
		Source:    sourceName,
	}, func(i btree.Item) bool {
		kv := i.(*KV)
		if kv.Set != Endpoints || kv.Namespace != namespace || kv.Name != "" || kv.Source != sourceName {
			return false
		}

		ei := kv.Endpoint

		toDel = append(toDel, kv, &KV{
			Set:       Endpoints,
			Namespace: namespace,
			Name:      ei.ServiceName,
			Source:    sourceName,
			Key:       kv.Key,
		})

		return true
	})

	for _, toDel := range toDel {
		tx.del(toDel)
	}
}

func (tx *Tx) SetEndpoint(ei *localnetv1.EndpointInfo) {
	tx.roPanic()

	newHash := tx.s.hashOf(&localnetv1.EndpointInfo{
		Endpoint:   ei.Endpoint,
		Conditions: ei.Conditions,
		Topology:   ei.Topology,
	})

	if ei.Hash == newHash {
		return // not changed
	}

	prevKey := strconv.FormatUint(ei.Hash, 16)

	tx.del(&KV{
		Set:       Endpoints,
		Namespace: ei.Namespace,
		Name:      ei.ServiceName,
		Source:    ei.SourceName,
		Key:       prevKey,
	})

	// also delete by source only
	tx.del(&KV{
		Set:       Endpoints,
		Namespace: ei.Namespace,
		Source:    ei.SourceName,
		Key:       prevKey,
	})

	// update key
	ei.Hash = newHash
	key := strconv.FormatUint(ei.Hash, 16)

	tx.set(&KV{
		Set:       Endpoints,
		Namespace: ei.Namespace,
		Name:      ei.ServiceName,
		Source:    ei.SourceName,
		Key:       key,
		Value:     ei,
		Endpoint:  ei,
	})

	// also index by source only
	tx.set(&KV{
		Set:       Endpoints,
		Namespace: ei.Namespace,
		Source:    ei.SourceName,
		Key:       key,
		Value:     ei,
		Endpoint:  ei,
	})

}

// Nodes funcs

func (tx *Tx) GetNode(name string) *localnetv1.Node {
	i := tx.s.tree.Get(&KV{Set: Nodes, Name: name})

	if i == nil {
		return nil
	}

	return i.(*KV).Node.Node
}

func (tx *Tx) SetNode(n *localnetv1.Node) {
	ni := &localnetv1.NodeInfo{
		Node: n,
		Hash: tx.s.hashOf(n),
	}

	tx.set(&KV{
		Set:   Nodes,
		Name:  n.Name,
		Node:  ni,
		Value: ni,
	})
}

func (tx *Tx) DelNode(name string) {
	tx.del(&KV{
		Set:  Nodes,
		Name: name,
	})
}
