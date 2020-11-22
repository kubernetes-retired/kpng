package diffstore

import (
	"bytes"

	"github.com/google/btree"
)

type KV struct {
	Key   []byte
	Value interface{}
}

func (a *KV) Less(bItem btree.Item) bool {
	b := bItem.(*KV)
	return bytes.Compare(a.Key, b.Key) < 0
}

type ItemState int

const (
	ItemDeleted   ItemState = iota
	ItemChanged             = 1
	ItemUnchanged           = 2
)

type storeKV struct {
	key   []byte
	hash  uint64
	value interface{}
	state ItemState
}

func (a *storeKV) Less(bItem btree.Item) bool {
	b := bItem.(*storeKV)

	return bytes.Compare(a.key, b.key) < 0
}
