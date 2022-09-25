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

	"golang.org/x/exp/constraints"
	"github.com/cespare/xxhash"
)

func NewBufferStore[K constraints.Ordered]() *Store[K, *BufferLeaf] {
	return New[K](NewBufferLeaf)
}

type BufferLeaf struct {
	bytes.Buffer
}

func NewBufferLeaf() *BufferLeaf {
	return &BufferLeaf{bytes.Buffer{}}
}

var _ Leaf = NewBufferLeaf()

func (l *BufferLeaf) Reset() {
	l.Buffer.Reset()
}

func (l *BufferLeaf) Hash() uint64 {
	return xxhash.Sum64(l.Bytes())
}

func (l *BufferLeaf) Writeln() {
	l.WriteByte('\n')
}
