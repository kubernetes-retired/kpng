package proxystore

import (
	"fmt"
	"github.com/google/btree"
	"sigs.k8s.io/kpng/api/globalv1"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/server/serde"
	"strconv"
)

// Tx is a "Transaction".  It represents a change to the Kubernetes data model
// which needs to get incrementally reflected into the KPNG data model.
type Tx struct {
	// proxyStore is the store that this Tx lives in.
	proxyStore *Store
	ro         bool

	// isChanged determines wether we need to do work, it is currently used as a boolean value in the code base.
	// someday, however, we may derive value out of the total number of changes.
	// TODO decide if isChanged should just be a boolean, since we dont care about it's value.
	isChanged uint
}

func (tx *Tx) roPanic() {
	if tx.ro {
		panic("read-only!")
	}
}

// Each iterates over each item in the given set, stopping if the callback returns false
func (tx *Tx) Each(set Set, callback func(*BTreeItem) bool) {
	tx.proxyStore.tree.AscendGreaterOrEqual(&BTreeItem{Set: set}, func(i btree.Item) bool {
		kv := i.(*BTreeItem)

		if kv.Set != set {
			return false
		}

		return callback(kv)
	})
}

// Reset clears the store data
func (tx *Tx) Reset() {
	tx.roPanic()

	if tx.proxyStore.tree.Len() != 0 {
		tx.proxyStore.tree.Clear(false)
		tx.isChanged++
	}

	for set, isSync := range tx.proxyStore.sync {
		if isSync {
			tx.proxyStore.sync[set] = false
			tx.isChanged++
		}
	}
}

// set "upserts" an object into the underlying data model'proxyStore BTree.   The input BTreeItem must have it'proxyStore underlying
// type "Set" (ServiceSet, EndpointsSet, ...) set that it can be iterated and filtered against later on,
// for example, when iterating over all Services, all Endpoints, and so on...
func (tx *Tx) set(kv *BTreeItem) {

	// TODO: Shouldn't we validate the kv object'proxyStore essential fields (like the "Set" field) are non nil on insert?

	tx.roPanic()
	prev := tx.proxyStore.tree.Get(kv)

	// nothing to update...
	if prev != nil && prev.(*BTreeItem).Value.GetHash() == kv.Value.GetHash() {
		return // not changed
	}

	tx.proxyStore.tree.ReplaceOrInsert(kv)
	tx.isChanged++
}

// del deletes an object from the underlying data model'proxyStore BTree.
func (tx *Tx) del(kv *BTreeItem) {
	tx.roPanic()
	i := tx.proxyStore.tree.Delete(kv)
	if i != nil {
		tx.isChanged++
	}
}

// SetRaw creates a new BTreeItem object for the underlying BTree and sets it for you.  This can be used,
// for example, in cases where we are reading from another KPNG instance.
func (tx *Tx) SetRaw(set Set, path string, value Hashed) {
	kv := &BTreeItem{}
	kv.Set = set
	kv.SetFromPath(path)

	kv.Value = value

	switch v := value.(type) {
	case *globalv1.NodeInfo:
		kv.Node = v
		tx.set(kv)

	case *globalv1.ServiceInfo:
		kv.Service = v
		tx.set(kv)

	case *globalv1.EndpointInfo:
		kv.Endpoint = v
		tx.set(kv)

	default:
		panic(fmt.Errorf("unknown value type: %t", v))
	}
}

func (tx *Tx) DelRaw(set Set, path string) {
	kv := &BTreeItem{}
	kv.Set = set
	kv.SetFromPath(path)

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
	return tx.proxyStore.sync[set]
}

func (tx *Tx) SetSync(set Set) {
	tx.roPanic()

	if !tx.proxyStore.sync[set] {
		tx.proxyStore.sync[set] = true
		tx.isChanged++
	}
}

// Services funcs

func (tx *Tx) SetService(s *localv1.Service) {
	si := &globalv1.ServiceInfo{
		Service: s,
		Hash: serde.Hash(&globalv1.ServiceInfo{
			Service: s,
		}),
	}

	tx.set(&BTreeItem{
		Set:       Services,
		Namespace: s.Namespace,
		Name:      s.Name,
		Value:     si,
		Service:   si,
	})
}

func (tx *Tx) DelService(namespace, name string) {
	tx.del(&BTreeItem{
		Set:       Services,
		Namespace: namespace,
		Name:      name,
	})
}

// Endpoints funcs

func (tx *Tx) EachEndpointOfService(namespace, serviceName string, callback func(*globalv1.EndpointInfo)) {
	tx.proxyStore.tree.AscendGreaterOrEqual(&BTreeItem{
		Set:       Endpoints,
		Namespace: namespace,
		Name:      serviceName,
	}, func(i btree.Item) bool {
		kv := i.(*BTreeItem)

		if kv.Set != Endpoints || kv.Namespace != namespace || kv.Name != serviceName {
			return false
		}

		callback(kv.Endpoint)

		return true
	})
}

// SetEndpointsOfSource replaces ALL endpoints of a single source (add new, update existing, delete removed)
func (tx *Tx) SetEndpointsOfSource(namespace, sourceName string, eis []*globalv1.EndpointInfo) {
	tx.roPanic()

	seen := map[uint64]bool{}

	for _, ei := range eis {
		if ei.Namespace != namespace {
			panic("inconsistent namespace: " + namespace + " != " + ei.Namespace)
		}
		if ei.SourceName != sourceName {
			panic("inconsistent source: " + sourceName + " != " + ei.SourceName)
		}

		ei.Hash = serde.Hash(&globalv1.EndpointInfo{
			Endpoint:   ei.Endpoint,
			Conditions: ei.Conditions,
			Topology:   ei.Topology,
		})
		seen[ei.Hash] = true
	}

	// to delete unseen endpoints
	toDel := make([]*BTreeItem, 0)

	tx.proxyStore.tree.AscendGreaterOrEqual(&BTreeItem{
		Set:       Endpoints,
		Namespace: namespace,
		Source:    sourceName,
	}, func(i btree.Item) bool {
		kv := i.(*BTreeItem)
		if kv.Set != Endpoints || kv.Namespace != namespace || kv.Source != sourceName {
			return false
		}

		if !seen[kv.Endpoint.Hash] {
			ei := kv.Endpoint
			toDel = append(toDel,
				kv,
				&BTreeItem{ // also remove the reference in the service
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

		kv := &BTreeItem{
			Set:       Endpoints,
			Namespace: ei.Namespace,
			Name:      ei.ServiceName,
			Source:    ei.SourceName,
			Key:       key,
			Value:     ei,
			Endpoint:  ei,
		}

		if tx.proxyStore.tree.Has(kv) {
			continue
		}

		tx.set(kv)

		// also index by source only
		tx.set(&BTreeItem{
			Set:       Endpoints,
			Namespace: ei.Namespace,
			Source:    ei.SourceName,
			Key:       key,
			Value:     ei,
			Endpoint:  ei,
		})
	}
}

// DelEndpointsOfSource finds all endpoints that need to be deleted, and
// writes transactions for deleting all of them.
func (tx *Tx) DelEndpointsOfSource(namespace, sourceName string) {
	tx.roPanic()

	toDel := make([]*BTreeItem, 0)

	tx.proxyStore.tree.AscendGreaterOrEqual(&BTreeItem{
		Set:       Endpoints,
		Namespace: namespace,
		Source:    sourceName,
	}, func(i btree.Item) bool {
		kv := i.(*BTreeItem)
		if kv.Set != Endpoints || kv.Namespace != namespace || kv.Name != "" || kv.Source != sourceName {
			return false
		}

		ei := kv.Endpoint

		toDel = append(toDel, kv, &BTreeItem{
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

func (tx *Tx) SetEndpoint(ei *globalv1.EndpointInfo) {
	tx.roPanic()

	newHash := serde.Hash(&globalv1.EndpointInfo{
		Endpoint:   ei.Endpoint,
		Conditions: ei.Conditions,
		Topology:   ei.Topology,
	})

	if ei.Hash == newHash {
		return // not changed
	}

	prevKey := strconv.FormatUint(ei.Hash, 16)

	tx.del(&BTreeItem{
		Set:       Endpoints,
		Namespace: ei.Namespace,
		Name:      ei.ServiceName,
		Source:    ei.SourceName,
		Key:       prevKey,
	})

	// also delete by source only
	tx.del(&BTreeItem{
		Set:       Endpoints,
		Namespace: ei.Namespace,
		Source:    ei.SourceName,
		Key:       prevKey,
	})

	// update key
	ei.Hash = newHash
	key := strconv.FormatUint(ei.Hash, 16)

	tx.set(&BTreeItem{
		Set:       Endpoints,
		Namespace: ei.Namespace,
		Name:      ei.ServiceName,
		Source:    ei.SourceName,
		Key:       key,
		Value:     ei,
		Endpoint:  ei,
	})

	// also index by source only
	tx.set(&BTreeItem{
		Set:       Endpoints,
		Namespace: ei.Namespace,
		Source:    ei.SourceName,
		Key:       key,
		Value:     ei,
		Endpoint:  ei,
	})

}

// Nodes funcs

func (tx *Tx) GetNode(name string) *globalv1.Node {
	i := tx.proxyStore.tree.Get(&BTreeItem{Set: Nodes, Name: name})

	if i == nil {
		return nil
	}

	return i.(*BTreeItem).Node.Node
}

func (tx *Tx) SetNode(n *globalv1.Node) {
	ni := &globalv1.NodeInfo{
		Node: n,
		Hash: serde.Hash(n),
	}

	tx.set(&BTreeItem{
		Set:   Nodes,
		Name:  n.Name,
		Node:  ni,
		Value: ni,
	})
}

func (tx *Tx) DelNode(name string) {
	tx.del(&BTreeItem{
		Set:  Nodes,
		Name: name,
	})
}
