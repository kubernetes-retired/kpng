package proxystore

import (
	"strconv"
	"sync"

	"github.com/cespare/xxhash"
	"github.com/gogo/protobuf/proto"
	"github.com/google/btree"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

type Store struct {
	sync.RWMutex
	c    *sync.Cond
	rev  uint64
	tree *btree.BTree

	// set sync info
	sync map[Set]bool

	pb *proto.Buffer
}

type Set int

const (
	Services Set = iota
	Endpoints
	Nodes
)

type KV struct {
	Sync      *bool
	Set       Set
	Namespace string
	Name      string
	Source    string
	Key       string

	Service      *localnetv1.Service
	TopologyKeys []string

	Endpoint *localnetv1.EndpointInfo
	Node     *v1.Node
}

func (a *KV) Less(i btree.Item) bool {
	b := i.(*KV)

	if a.Set != b.Set {
		return a.Set < b.Set
	}
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	if a.Source != b.Source {
		return a.Source < b.Source
	}

	return a.Key < b.Key
}

func New() *Store {
	return &Store{
		c:    sync.NewCond(&sync.Mutex{}),
		tree: btree.New(2),
		sync: map[Set]bool{},
		pb:   proto.NewBuffer(make([]byte, 0)),
	}
}

func (s *Store) Update(update func(tx *Tx)) {
	s.Lock()
	defer s.Unlock()

	update(&Tx{s, false})

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

func (s *Store) View(afterRev uint64, view func(tx *Tx)) uint64 {
	s.c.L.Lock()
	for s.rev <= afterRev {
		s.c.Wait()
	}
	s.c.L.Unlock()

	s.RLock()
	defer s.RUnlock()

	view(&Tx{s, true})

	return s.rev
}

type Tx struct {
	s  *Store
	ro bool
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
	tx.s.sync[set] = true
}

// Services funcs

func (tx *Tx) SetService(s *localnetv1.Service, topologyKeys []string) {
	tx.roPanic()
	tx.s.tree.ReplaceOrInsert(&KV{
		Set:          Services,
		Namespace:    s.Namespace,
		Name:         s.Name,
		Service:      s,
		TopologyKeys: topologyKeys,
	})
}

func (tx *Tx) DelService(s *v1.Service) {
	tx.roPanic()
	tx.s.tree.Delete(&KV{
		Set:       Services,
		Namespace: s.Namespace,
		Name:      s.Name,
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

	seen := map[string]bool{}

	for _, ei := range eis {
		if ei.Namespace != namespace {
			panic("inconsistent namespace: " + namespace + " != " + ei.Namespace)
		}
		if ei.SourceName != sourceName {
			panic("inconsistent source: " + sourceName + " != " + ei.SourceName)
		}
		if ei.EndpointHash == "" {
			if err := tx.s.pb.Marshal(ei.Endpoint); err != nil {
				panic(err) // should not happen
			}

			ei.EndpointHash = strconv.FormatUint(xxhash.Sum64(tx.s.pb.Bytes()), 16)
		}

		seen[ei.EndpointHash] = true
	}

	// delete unseen endpoints
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

		if !seen[kv.Key] {
			ei := kv.Endpoint
			toDel = append(toDel, kv, &KV{
				Set:       Endpoints,
				Namespace: ei.Namespace,
				Name:      ei.ServiceName,
				Source:    ei.SourceName,
				Key:       ei.EndpointHash,
			})
		}

		return true
	})

	for _, toDel := range toDel {
		tx.s.tree.Delete(toDel)
	}

	// add/update known endpoints
	for _, ei := range eis {
		seen[ei.EndpointHash] = true

		tx.s.tree.ReplaceOrInsert(&KV{
			Set:       Endpoints,
			Namespace: ei.Namespace,
			Name:      ei.ServiceName,
			Source:    ei.SourceName,
			Key:       ei.EndpointHash,
			Endpoint:  ei,
		})

		// also index by source only
		tx.s.tree.ReplaceOrInsert(&KV{
			Set:       Endpoints,
			Namespace: ei.Namespace,
			Source:    ei.SourceName,
			Key:       ei.EndpointHash,
			Endpoint:  ei,
		})
	}
}

func (tx *Tx) DelEndpointsOfSource(namespace, sourceName string) {
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
		tx.s.tree.Delete(toDel)
	}
}

// Nodes funcs

func (tx *Tx) GetNode(name string) *v1.Node {
	i := tx.s.tree.Get(&KV{Set: Nodes, Name: name})

	if i == nil {
		return nil
	}

	return i.(*KV).Node
}

func (tx *Tx) SetNode(n *v1.Node) {
	tx.roPanic()
	tx.s.tree.ReplaceOrInsert(&KV{
		Set:  Nodes,
		Name: n.Name,
		Node: n,
	})
}

func (tx *Tx) DelNode(n *v1.Node) {
	tx.roPanic()
	tx.s.tree.Delete(&KV{
		Set:  Nodes,
		Name: n.Name,
	})
}
