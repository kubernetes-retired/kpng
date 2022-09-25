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
	"golang.org/x/exp/constraints"
	"github.com/google/btree"
)

type Item[K constraints.Ordered, V Leaf] struct {
	k K
	v V

	touched      bool
	previousHash uint64
	currentHash  uint64

	deferred []func(V)
}

func (i1 *Item[K, V]) Less(i btree.Item) bool {
	i2 := i.(*Item[K, V])
	return i1.k < i2.k
}

func (i *Item[K, V]) Key() K {
	return i.k
}

func (i *Item[K, V]) Value() V {
	return i.v
}

func (i *Item[K, V]) Changed() bool {
	return i.Created() || i.Updated()
}

func (i *Item[K, V]) Created() bool {
	return i.touched && i.previousHash == 0
}

func (i *Item[K, V]) Updated() bool {
	return i.touched && i.previousHash != i.currentHash
}

func (i *Item[K, V]) Deleted() bool {
	return !i.touched && i.previousHash != 0
}

func (i *Item[K, V]) Defer(f func(V)) {
	i.deferred = append(i.deferred, f)
}

func (i *Item[K, V]) RunDeferred() {
	if len(i.deferred) == 0 {
		return
	}

	for _, f := range i.deferred {
		f(i.v)
	}

	i.deferred = i.deferred[:0]
}
