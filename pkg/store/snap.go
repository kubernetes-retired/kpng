package store

import (
	"github.com/golang/protobuf/proto"
	"github.com/google/btree"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var NotFound = grpc.Errorf(codes.NotFound, "key not found")

type Snapshot struct {
	rev  uint64
	tree *btree.BTree
}

type KV struct {
	// Err indicate an error was encountered while streaming values.
	Err error
	// Key where to the value
	Key []byte
	// Value itself
	Value proto.Message
}

func (s *Snapshot) Rev() uint64 {
	return s.rev
}

func (s *Snapshot) Iterate(newValue func() proto.Message) <-chan KV {
	ch := make(chan KV, 10)

	go func() {
		err := s.Visit(newValue, func(key []byte, value proto.Message) error {
			ch <- KV{Key: key, Value: value}
			return nil
		})
		if err != nil {
			ch <- KV{Err: err}
		}
		close(ch)
	}()

	return ch
}

func (s *Snapshot) Visit(newValue func() proto.Message, callback func(key []byte, value proto.Message) error) (err error) {
	s.tree.Ascend(func(i btree.Item) bool {
		item := i.(storeItem)

		value := newValue()
		err = proto.Unmarshal(item.value, value)
		if err == nil {
			err = callback(item.key, value)
		}
		return err == nil
	})
	return
}

func (s *Snapshot) VisitRaw(callback func(key, value []byte)) {
	s.tree.Ascend(func(i btree.Item) bool {
		item := i.(storeItem)
		callback(item.key, item.value)
		return true
	})
}

func (s *Snapshot) Get(key []byte, value proto.Message) (err error) {
	i := s.tree.Get(storeItem{key: key})

	if i == nil {
		return NotFound
	}

	item := i.(storeItem)
	err = proto.Unmarshal(item.value, value)
	return
}
