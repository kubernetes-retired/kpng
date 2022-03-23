package diffstore2

import (
	"encoding/json"

	"golang.org/x/exp/constraints"
	"github.com/cespare/xxhash"
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
