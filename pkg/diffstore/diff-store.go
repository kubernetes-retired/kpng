package diffstore

import (
	"bytes"
	"sync"

	"github.com/google/btree"
)

const DefaultSeparator = '|'

// DiffStore tracks changes by prefix and sub keys
type DiffStore struct {
	l    sync.RWMutex
	sep  byte
	tree *btree.BTree
}

func New() *DiffStore {
	return NewWithSeparator(DefaultSeparator)
}

func NewWithSeparator(separator byte) *DiffStore {
	return &DiffStore{tree: btree.New(2), sep: separator}
}

func (s *DiffStore) ListPrefixes() (prefixes [][]byte) {
	s.l.RLock()
	defer s.l.RUnlock()

	prefixes = make([][]byte, 0)

	var last []byte
	s.tree.Ascend(func(i btree.Item) bool {
		kv := i.(*storeKV)
		prefix := kv.key[:bytes.IndexByte(kv.key, s.sep)]
		if !bytes.Equal(last, prefix) {
			prefixes = append(prefixes, prefix)
			last = prefix
		}
		return true
	})

	return
}

func (s *DiffStore) GetValues(prefix []byte) (values []interface{}) {
	s.l.RLock()
	defer s.l.RUnlock()

	values = make([]interface{}, 0)

	fullPrefix := s.fullPrefix(prefix)
	s.tree.AscendGreaterOrEqual(storeKV{key: fullPrefix}, func(i btree.Item) bool {
		kv := i.(storeKV)

		if !bytes.HasPrefix(kv.key, fullPrefix) {
			return false
		}

		values = append(values, kv.value)
		return true
	})

	return
}

func (s *DiffStore) getByPrefix(prefix []byte) (kvs storeKVs) {
	kvs = make(storeKVs, 0)
	s.tree.AscendGreaterOrEqual(storeKV{key: prefix}, func(i btree.Item) bool {
		kv := i.(storeKV)

		if !bytes.HasPrefix(kv.key, prefix) {
			return false
		}

		kvs = append(kvs, kv)
		return true
	})
	return
}

func (s *DiffStore) Begin(prefix []byte) *StoreUpdate {
	return &StoreUpdate{s, prefix, make(storeKVs, 0)}
}

func (s *DiffStore) set(prefix []byte, elements storeKVs) (changes Changes) {
	s.l.Lock()
	defer s.l.Unlock()

	changes.Prefix = string(prefix)

	fullPrefix := s.fullPrefix(prefix)
	prevElements := s.getByPrefix(fullPrefix)

	// check for updates or deletions
	updates := make(storeKVs, 0, len(elements))
	for _, prev := range prevElements {
		subKey := prev.key[len(fullPrefix):]
		element := elements.Find(subKey)

		if element == nil {
			s.tree.Delete(prev)
			changes.delete(prev.value)
			continue
		}

		if prev.hash == element.hash {
			// not updated
			continue
		}

		updates = append(updates, storeKV{prev.key, element.hash, element.value})
	}

	// check for new values
	for _, element := range elements {
		key := s.fullKey(prefix, element.key)
		prev := prevElements.Find(key)

		if prev == nil {
			updates = append(updates, storeKV{key, element.hash, element.value})
		}
	}

	for _, element := range updates {
		s.tree.ReplaceOrInsert(element)
		changes.set(element.value)
	}

	return
}

func (s *DiffStore) Delete(prefix []byte) (changes Changes) {
	s.l.Lock()
	defer s.l.Unlock()

	changes.Prefix = string(prefix)

	for _, elt := range s.getByPrefix(s.fullPrefix(prefix)) {
		s.tree.Delete(elt)
		changes.delete(elt.value)
	}

	return
}

func (s *DiffStore) fullPrefix(prefix []byte) []byte {
	return append(append(make([]byte, 0, len(prefix)+1), prefix...), s.sep)
}

func (s *DiffStore) fullKey(prefix, key []byte) []byte {
	return append(append(append(make([]byte, 0, len(prefix)+1+len(key)),
		prefix...), s.sep), key...)
}

func (s *DiffStore) DumpTo(values chan<- interface{}) {
	s.l.RLock()
	defer s.l.RUnlock()

	s.tree.Ascend(func(i btree.Item) bool {
		values <- i.(storeKV).value
		return true
	})
}
