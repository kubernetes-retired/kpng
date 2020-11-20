package diffstore

import (
	"github.com/google/btree"
)

const DefaultSeparator = '|'

// DiffStore tracks changes by prefix and sub keys
type DiffStore struct {
	tree *btree.BTree
}

func New() *DiffStore {
	return &DiffStore{tree: btree.New(2)}
}

// Reset the store to clear, marking all entries as deleted (and removing previously deleted ones)
func (s *DiffStore) Reset() {
	toDelete := make([]*storeKV, 0)

	s.tree.Ascend(func(i btree.Item) bool {
		v := i.(*storeKV)
		if v.state == itemDeleted {
			toDelete = append(toDelete, v)
		} else {
			v.state = itemDeleted
		}
		return true
	})

	for _, i := range toDelete {
		s.tree.Delete(i)
	}
}

func (s *DiffStore) Set(key []byte, hash uint64, value interface{}) {
	item := s.tree.Get(&storeKV{key: key})

	if item == nil {
		s.tree.ReplaceOrInsert(&storeKV{
			key:   key,
			hash:  hash,
			value: value,
			state: itemSet,
		})
		return
	}

	v := item.(*storeKV)

	if v.hash == hash {
		if v.state == itemDeleted {
			v.state = itemUnchanged
		}
		return
	}

	v.hash = hash
	v.value = value
	v.state = itemSet
}

func (s *DiffStore) Updated() (updated []KV) {
	s.tree.Ascend(func(i btree.Item) bool {
		v := i.(*storeKV)

		if v.state == itemSet {
			updated = append(updated, KV{v.key, v.value})
		}

		return true
	})
	return
}

func (s *DiffStore) Deleted() (deleted []KV) {
	s.tree.Descend(func(i btree.Item) bool {
		v := i.(*storeKV)

		if v.state == itemDeleted {
			deleted = append(deleted, KV{v.key, v.value})
		}

		return true
	})
	return
}
