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
	"fmt"

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

func (a KV) String() string {
	return fmt.Sprintf("{%s => %v}", string(a.Key), a.Value)
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
