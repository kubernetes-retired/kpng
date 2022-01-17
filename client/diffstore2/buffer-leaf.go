package diffstore2

import (
	"bytes"
    "constraints"

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

func (l *BufferLeaf) Writeln()  {
    l.WriteByte('\n')
}
