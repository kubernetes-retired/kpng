package diffstore2

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
	if !l.equal(l.value, v) {
		l.hash++
	}

	l.value = v
}

func (l *AnyLeaf[T]) String() string {
	return fmt.Sprintf("{%v}", l.value)
}
