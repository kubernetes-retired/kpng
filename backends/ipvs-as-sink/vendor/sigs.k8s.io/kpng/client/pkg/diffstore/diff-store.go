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

package diffstore

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cespare/xxhash"
	"github.com/google/btree"
	"google.golang.org/protobuf/proto"
)

const DefaultSeparator = '|'

// DiffStore tracks changes by prefix and sub keys
type DiffStore struct {
	tree *btree.BTree
}

func New() *DiffStore {
	return &DiffStore{tree: btree.New(2)}
}

// Reset the store to clear, marking all entries with the given state (and removing previously deleted ones)
func (s *DiffStore) Reset(state ItemState) {
	toDelete := make([]*storeKV, 0)

	s.tree.Ascend(func(i btree.Item) bool {
		v := i.(*storeKV)
		if v.state == ItemDeleted {
			// previous deleted items are removed
			toDelete = append(toDelete, v)
		} else {
			v.state = state
			// XXX now we have Get*, we can't remove the value anymore // v.value = nil
		}
		return true
	})

	for _, i := range toDelete {
		s.tree.Delete(i)
	}
}

// Set insert or update a key/value in the store
func (s *DiffStore) Set(key []byte, hash uint64, value interface{}) {
	item := s.tree.Get(&storeKV{key: key})

	if item == nil {
		s.tree.ReplaceOrInsert(&storeKV{
			key:   key,
			hash:  hash,
			value: value,
			state: ItemChanged,
		})
		return
	}

	v := item.(*storeKV)

	if v.hash == hash {
		if v.state == ItemDeleted {
			v.value = value
			v.state = ItemUnchanged
		}
		return
	}

	v.hash = hash
	v.value = value
	v.state = ItemChanged
}

// SetJSON is a helper that calls Set with hash of the JSON representation of value
func (s *DiffStore) SetJSON(key []byte, value interface{}) {
	valueBytes, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Errorf("failed to JSON marshal value: %w", err))
	}

	h := xxhash.Sum64(valueBytes)
	s.Set(key, h, value)
}

// SetProto is a helper that calls Set with hash of the protobuf representation of value
func (s *DiffStore) SetProto(key []byte, value proto.Message) {
	valueBytes, err := proto.Marshal(value)
	if err != nil {
		panic(fmt.Errorf("failed to proto marshal value: %w", err))
	}

	h := xxhash.Sum64(valueBytes)
	s.Set(key, h, value)
}

// Delete an entry from the store
func (s *DiffStore) Delete(key []byte) {
	item := s.tree.Get(&storeKV{key: key})
	item.(*storeKV).state = ItemDeleted
}

// DeleteByPrefix removes all entries with the given prefix from the store
func (s *DiffStore) DeleteByPrefix(prefix []byte) {
	s.tree.AscendGreaterOrEqual(&storeKV{key: prefix}, func(i btree.Item) bool {
		v := i.(*storeKV)

		if !bytes.HasPrefix(v.key, prefix) {
			return false
		}

		v.state = ItemDeleted

		return true
	})
}

// Updated returns all the entries that where updated since the last Reset.
func (s *DiffStore) Updated() (updated []KV) {
	s.tree.Ascend(func(i btree.Item) bool {
		v := i.(*storeKV)

		if v.state == ItemChanged {
			updated = append(updated, KV{v.key, v.value})
		}

		return true
	})
	return
}

// Updated returns all the entries that where deleted since the last Reset.
func (s *DiffStore) Deleted() (deleted []KV) {
	s.tree.Descend(func(i btree.Item) bool {
		v := i.(*storeKV)

		if v.state == ItemDeleted {
			deleted = append(deleted, KV{v.key, v.value})
		}

		return true
	})
	return
}

// GetByPrefix returns all the entries with the given prefix.
func (s *DiffStore) GetByPrefix(prefix []byte) (items []KV) {
	s.tree.AscendGreaterOrEqual(&storeKV{key: prefix}, func(i btree.Item) bool {
		v := i.(*storeKV)

		if !bytes.HasPrefix(v.key, prefix) {
			return false
		}

		if v.state == ItemDeleted {
			return true
		}

		items = append(items, KV{v.key, v.value})

		return true
	})

	return
}
