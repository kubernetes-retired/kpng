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

func ExampleAnyLeafStore() {
	store := NewAnyStore[string](func(a, b string) bool { return a == b })

	store.Get("a").Set("a1")
	store.Done()
	store.printDiff()

	store.Reset()
	store.Get("a").Set("a1")
	store.Done()
	store.printDiff()

	store.Reset()
	store.Get("a").Set("a2")
	store.Done()
	store.printDiff()

	store.Reset()
	store.Done()
	store.printDiff()

	// Output:
	// -----
	// C a => "{a1}"
	// -----
	// <same>
	// -----
	// U a => "{a2}"
	// -----
	// D a
}

func ExampleAnyLeafStoreFileLike() {
	store := NewAnyStore[int](func(a, b string) bool { return a == b })

	// File 1
	// line1
	// line2

	store.Get(1).Set("line1")
	store.Get(2).Set("line2")
	store.Done()
	store.printDiff()

	store.Reset()

	// File 2
	// line2.1
	// line2.2

	store.Get(1).Set("line2.1")
	store.Get(2).Set("line2.2")
	store.Done()
	store.printDiff()

	// Output:
	// -----
	// C 1 => "{line1}"
	// C 2 => "{line2}"
	// -----
	// U 1 => "{line2.1}"
	// U 2 => "{line2.2}"
}
