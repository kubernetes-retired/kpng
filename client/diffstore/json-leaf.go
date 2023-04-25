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
	"encoding/json"

	"github.com/cespare/xxhash"
	"golang.org/x/exp/constraints"
)

func NewJSONStore[K constraints.Ordered, T any]() *Store[K, *JSONLeaf[T]] {
	return New[K](NewJSONLeaf[T])
}

type JSONLeaf[T any] struct {
	value T
}

func NewJSONLeaf[T any]() *JSONLeaf[T] {
	return &JSONLeaf[T]{}
}

var _ Leaf = NewJSONLeaf[any]()

func (l *JSONLeaf[T]) Get() T {
	return l.value
}

func (l *JSONLeaf[T]) Set(v T) {
	l.value = v
}

func (l *JSONLeaf[T]) Reset() {
	l.value = JSONLeaf[T]{}.value
}

func (l *JSONLeaf[T]) Hash() uint64 {
	ba, err := json.Marshal(l.value)
	if err != nil {
		panic(err)
	}
	return xxhash.Sum64(ba)
}

func (l *JSONLeaf[T]) String() string {
	ba, _ := json.Marshal(l.value)
	return string(ba)
}
