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
	"sync"

	"github.com/google/btree"
	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/server/pkg/metrics"
)

// proxystore stores information in KPNG'proxyStore data model for kubernetes Services, Endpoints, and Nodes, which
// are the core objects associated with the "kube proxy".

// Store has the entire state space of the KPNG data model managed as a Btree.  See the BtreeItem for details
// on how the services, endpoints, and nodes are sorted into the underlying tree.  When new networking info comes
// in, use the Store's Update() function to safely update the underlying datamodel.
type Store struct {
	sync.RWMutex
	c      *sync.Cond
	rev    uint64
	closed bool
	tree   *btree.BTree

	// set sync info
	sync map[Set]bool
}

type Set = localv1.Set

const (
	Services  = localv1.Set_GlobalServiceInfos
	Endpoints = localv1.Set_GlobalEndpointInfos
	Nodes     = localv1.Set_GlobalNodeInfos
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

func (s *Store) Close() {
	s.c.L.Lock()
	s.closed = true
	s.c.Broadcast()
	s.c.L.Unlock()
}

// Update is the function we use to lock and unlock the store
// before we add/delete a new object from it.
func (s *Store) Update(update func(tx *Tx)) {
	s.Lock()
	defer s.Unlock()

	metrics.Kpng_k8s_api_events.Inc()

	tx := &Tx{proxyStore: s}
	update(tx)

	if tx.isChanged == 0 {
		return // nothing changed
	}

	// TODO check if the update really updated something
	s.c.L.Lock()
	s.rev++

	// TODO explain why we call Broadcast?
	s.c.Broadcast()
	s.c.L.Unlock()

	if log := klog.V(3); log.Enabled() {
		klog.Info("store updated to rev ", s.rev, " with ", s.tree.Len(), " entries")
		if log := klog.V(4); log.Enabled() {
			// Iterate the btree
			s.tree.Ascend(func(i btree.Item) bool {
				kv := i.(*BTreeItem)
				log.Info("- entry: ", kv.Sync, "/", kv.Set, ": ", kv.Namespace, "/", kv.Name, "/", kv.Source, "/", kv.Key)
				return true
			})
		}
	}
}

// View visits all nodes of the underlying stores tree and allows you to see all the transactions
// that have happened.
func (s *Store) View(afterRev uint64, view func(tx *Tx)) (rev uint64, closed bool) {
	s.c.L.Lock()
	// try to unlock
	for s.rev <= afterRev && !s.closed {
		s.c.Wait()
	}
	s.c.L.Unlock()

	s.RLock()
	defer s.RUnlock()

	if s.closed {
		return 0, s.closed
	}

	view(&Tx{proxyStore: s, ro: true})

	return s.rev, s.closed
}
