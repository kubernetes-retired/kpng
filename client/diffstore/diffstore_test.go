package diffstore

import (
	"fmt"
	"testing"
)

func ExampleStore() {
	store := NewBufferStore[string]()

	{
		fmt.Fprint(store.Get("a"), "hello a")

		store.Done()
		store.printDiff()
	}

	{
		store.Reset()

		fmt.Fprint(store.Get("a"), "hello a")

		store.Done()
		store.printDiff()
	}

	{
		store.Reset()

		fmt.Fprint(store.Get("a"), "hello a")
		fmt.Fprint(store.Get("b"), "hello b")

		store.Done()
		store.printDiff()
	}

	{
		store.Reset()

		fmt.Fprint(store.Get("a"), "hi a")

		store.Done()
		store.printDiff()
	}

	{
		store.Reset()

		fmt.Fprint(store.Get("b"), "hi b")

		store.Done()
		store.printDiff()
	}

	{
		store.Reset()

		store.Done()
		store.printDiff()
	}

	// Output:
	// -----
	// C a => "hello a"
	// -----
	// <same>
	// -----
	// C b => "hello b"
	// -----
	// U a => "hi a"
	// D b
	// -----
	// C b => "hi b"
	// D a
	// -----
	// D b
}

func TestStoreCleanup(t *testing.T) {
	store := New[string](NewBufferLeaf)

	hasKey := func(k string) bool { return nil != store.data.Get(&Item[string, *BufferLeaf]{k: k}) }

	// write a node in the store
	store.Get("a").WriteString("hello")
	store.Done()

	// the node should persist after 1 reset
	store.Reset()
	store.Done()
	if !hasKey("a") {
		t.Error("key not found")
	}

	// the node should fade after 2 more resets
	store.Reset()
	store.Done()
	store.Reset()
	if hasKey("a") {
		t.Error("key still there")
	}
}
