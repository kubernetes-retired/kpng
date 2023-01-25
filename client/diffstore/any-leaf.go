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
	"fmt"

	"golang.org/x/exp/constraints"
)

func NewAnyStore[K constraints.Ordered, T any](equal func(a, b T) bool) *Store[K, *AnyLeaf[T]] {
	return New[K](func() *AnyLeaf[T] { return NewAnyLeaf(equal) })
}

type AnyLeaf[T any] struct {
	equal func(a, b T) bool
	value T

	hash uint64
}

func NewAnyLeaf[T any](equal func(a, b T) bool) *AnyLeaf[T] {
	return &AnyLeaf[T]{equal: equal, hash: 1}
}

var _ Leaf = NewAnyLeaf(func(a, b string) bool { return a == b })

func (l *AnyLeaf[T]) Reset() {
}

func (l *AnyLeaf[T]) Hash() uint64 {
	return l.hash
}

func (l *AnyLeaf[T]) Get() T {
	return l.value
}

func (l *AnyLeaf[T]) Set(v T) {
	// if the leaf is new (hash==1), don't call equal as we already know nothing is not equal to something
	if l.hash == 1 || !l.equal(l.value, v) {
		l.hash++
	}

	l.value = v
}

func (l *AnyLeaf[T]) String() string {
	return fmt.Sprintf("{%v}", l.value)
}
