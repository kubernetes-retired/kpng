package diffstore

import (
	"bytes"

	"github.com/google/btree"
)

type storeKV struct {
	key   []byte
	hash  uint64
	value interface{}
}

func (a storeKV) Less(bItem btree.Item) bool {
	b := bItem.(storeKV)

	return bytes.Compare(a.key, b.key) < 0
}

type storeKVs []storeKV

func (kvs storeKVs) Find(key []byte) *storeKV {
	for idx, kv := range kvs {
		if bytes.Equal(kv.key, key) {
			return &kvs[idx]
		}
	}
	return nil
}
