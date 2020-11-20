package diffstore

import (
	"bytes"

	"github.com/google/btree"
)

type KV struct {
	Key   []byte
	Value interface{}
}

type itemState int

const (
	itemDeleted   itemState = iota
	itemSet                 = 1
	itemUnchanged           = 2
)

type storeKV struct {
	key   []byte
	hash  uint64
	value interface{}
	state itemState
}

func (a *storeKV) Less(bItem btree.Item) bool {
	b := bItem.(*storeKV)

	return bytes.Compare(a.key, b.key) < 0
}
