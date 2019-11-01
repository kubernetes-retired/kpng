package store

import (
	"bytes"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/google/btree"
)

type Store struct {
	c    *sync.Cond
	rev  uint64
	tree *btree.BTree

	snap *Snapshot
}

func New() *Store {
	return &Store{
		c:    sync.NewCond(&sync.Mutex{}),
		rev:  0,
		tree: btree.New(2),
	}
}

type storeItem struct {
	key, value []byte
}

var _ btree.Item = storeItem{}

func (i storeItem) Less(bItem btree.Item) bool {
	return bytes.Compare(i.key, bItem.(storeItem).key) < 0
}

func (s *Store) Set(key []byte, value proto.Message) (err error) {
	i := storeItem{
		key: key,
	}

	if value != nil {
		i.value, err = proto.Marshal(value)
		if err != nil {
			return
		}
	}

	s.c.L.Lock()
	defer s.c.L.Unlock()

	current := s.tree.Get(i)

	// check if it's a real update
	if current == nil && value == nil {
		return
	}
	if current != nil && bytes.Equal(i.value, current.(storeItem).value) {
		return
	}

	// apply update
	if value == nil {
		s.tree.Delete(i)
	} else {
		s.tree.ReplaceOrInsert(i)
	}

	s.rev++

	s.c.Broadcast()
	return
}

func (s *Store) Next(rev uint64) (snap *Snapshot) {
	s.c.L.Lock()
	defer s.c.L.Unlock()

	for s.rev <= rev {
		s.c.Wait()
	}

	if s.snap == nil || s.snap.Rev() != s.rev {
		s.snap = &Snapshot{s.rev, s.tree.Clone()}
	}

	return s.snap
}
