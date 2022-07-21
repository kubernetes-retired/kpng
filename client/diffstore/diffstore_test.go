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
