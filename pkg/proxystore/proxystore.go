package proxystore

import (
	"fmt"
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

type Hashed interface {
	GetHash() uint64
}

type KV struct {
	Sync      *bool
	Set       Set
	Namespace string
	Name      string
	Source    string
	Key       string

	Service  *localnetv1.ServiceInfo
	Endpoint *localnetv1.EndpointInfo
	Node     *localnetv1.NodeInfo
}

func (kv *KV) Value() Hashed {
	switch kv.Set {
	case Services:
		return kv.Service
	case Endpoints:
		return kv.Endpoint
	case Nodes:
		return kv.Node
	}
	panic(fmt.Errorf("unknown set: %d", kv.Set)) // should not happen
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

func (s *Store) hashOf(m proto.Message) (h uint64) {
	if err := s.pb.Marshal(m); err != nil {
		panic(err) // should not happen
	}
	h = xxhash.Sum64(s.pb.Bytes())
	s.pb.Reset()
	return h
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

func (s *Store) View(afterRev uint64, view func(tx *Tx)) uint64 {
	s.c.L.Lock()
	for s.rev <= afterRev {
		s.c.Wait()
	}
	s.c.L.Unlock()

	s.RLock()
	defer s.RUnlock()

	view(&Tx{s: s, ro: true})

	return s.rev
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

func (tx *Tx) set(kv *KV) {
	tx.roPanic()
	prev := tx.s.tree.Get(kv)

	if prev != nil && prev.(*KV).Value().GetHash() == kv.Value().GetHash() {
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
		Hash:         tx.s.hashOf(s),
	}

	tx.set(&KV{
		Set:       Services,
		Namespace: s.Namespace,
		Name:      s.Name,
		Service:   si,
	})
}

func (tx *Tx) DelService(s *v1.Service) {
	tx.del(&KV{
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

	seen := map[uint64]bool{}

	for _, ei := range eis {
		if ei.Namespace != namespace {
			panic("inconsistent namespace: " + namespace + " != " + ei.Namespace)
		}
		if ei.SourceName != sourceName {
			panic("inconsistent source: " + sourceName + " != " + ei.SourceName)
		}

		ei.Hash = tx.s.hashOf(ei.Endpoint)
		seen[ei.Hash] = true
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
		Set:  Nodes,
		Name: n.Name,
		Node: ni,
	})
}

func (tx *Tx) DelNode(name string) {
	tx.del(&KV{
		Set:  Nodes,
		Name: name,
	})
}
