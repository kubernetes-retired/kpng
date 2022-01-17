package diffstore2

import (
	"constraints"

	"github.com/google/btree"
)

type Leaf interface {
	Reset()
	Hash() uint64
}

type Store[K constraints.Ordered, V Leaf] struct {
	data     *btree.BTree
	newValue func() V

	done    bool
	touched int
}

func New[K constraints.Ordered, V Leaf](newValue func() V) *Store[K, V] {
	return &Store[K, V]{
		data:     btree.New(4),
		newValue: newValue,
	}
}

func (s *Store[K, V]) GetItem(key K) *Item[K, V] {
	var item *Item[K, V]

	i := s.data.Get(&Item[K, V]{k: key})

	if i == nil {
		item = &Item[K, V]{k: key, v: s.newValue()}
		s.data.ReplaceOrInsert(item)

	} else {
		item = i.(*Item[K, V])
	}

	if !item.touched {
		item.touched = true
		s.touched++
	}

	return item
}

func (s *Store[K, V]) Get(key K) V {
	item := s.GetItem(key)
	return item.v
}

func (s *Store[K, V]) RunDeferred() {
	s.data.Ascend(func(item btree.Item) bool {
		i := item.(*Item[K, V])

		if i.touched {
			i.RunDeferred()
		}

		return true
	})
}

// Done must be called at the end of the filling process. It will compute hashes of every node to allow diff functions to work.
func (s *Store[K, V]) Done() {
	s.data.Ascend(func(item btree.Item) bool {
		i := item.(*Item[K, V])

		if i.touched {
			i.currentHash = i.v.Hash()
		}

		return true
	})

	s.done = true
}

func (s *Store[K, V]) List() (ret []*Item[K, V]) {
	ret = make([]*Item[K, V], 0, s.touched)

	s.data.Ascend(func(item btree.Item) bool {
		i := item.(*Item[K, V])

		if i.touched {
			ret = append(ret, i)
		}

		return true
	})

	return
}

func (s *Store[K, V]) Deleted() (ret []*Item[K, V]) {
	if !s.done {
		panic("Done() not called!")
	}

	ret = make([]*Item[K, V], 0, s.data.Len()-s.touched)

	s.data.Ascend(func(item btree.Item) bool {
		i := item.(*Item[K, V])

		if i.Deleted() {
			ret = append(ret, i)
		}

		return true
	})

	return
}

// Changed returns every entry that was created or updated
func (s *Store[K, V]) Changed() (ret []*Item[K, V]) {
	if !s.done {
		panic("Done() not called!")
	}

	ret = make([]*Item[K, V], 0, s.touched)

	s.data.Ascend(func(item btree.Item) bool {
		i := item.(*Item[K, V])

		if i.Changed() {
			ret = append(ret, i)
		}

		return true
	})

	return
}

func (s *Store[K, V]) HasChanges() (changed bool) {
	if !s.done {
		panic("Done() not called!")
	}

	s.data.Ascend(func(item btree.Item) bool {
		i := item.(*Item[K, V])
		changed = i.Changed()
		return !changed
	})

	return
}

func (s *Store[K, V]) Reset() {
	toDel := make([]*Item[K, V], 0)

	s.data.Ascend(func(item btree.Item) bool {
		i := item.(*Item[K, V])

		if i.previousHash == 0 && !i.touched {
			toDel = append(toDel, i)
			return true
		}

		i.previousHash = i.currentHash
		i.currentHash = 0
		i.touched = false

		i.v.Reset()

		return true
	})

	for _, item := range toDel {
		s.data.Delete(item)
	}

	s.done = false
	s.touched = 0
}
