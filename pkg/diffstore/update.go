package diffstore

import (
	"encoding/json"

	"github.com/cespare/xxhash"
)

type StoreUpdate struct {
	s        *DiffStore
	prefix   []byte
	elements storeKVs
}

func (u *StoreUpdate) Add(key []byte, hash uint64, value interface{}) {
	u.elements = append(u.elements, storeKV{key, hash, value})
}

func (u *StoreUpdate) AddJSON(key string, value interface{}) {
	h := xxhash.New()
	if err := json.NewEncoder(h).Encode(value); err != nil {
		panic(err)
	}

	u.Add([]byte(key), h.Sum64(), value)
}

func (u *StoreUpdate) Apply() Changes {
	return u.s.set(u.prefix, u.elements)
}
