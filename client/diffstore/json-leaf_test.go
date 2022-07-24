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

func ExampleJSONLeafStore() {
	type myT struct {
		V string
	}

	store := NewJSONStore[string, myT]()

	store.Get("a").Set(myT{"a1"})
	store.Done()
	store.printDiff()

	store.Reset()
	store.Get("a").Set(myT{"a1"})
	store.Done()
	store.printDiff()

	store.Reset()
	store.Get("a").Set(myT{"a2"})
	store.Done()
	store.printDiff()

	store.Reset()
	store.Done()
	store.printDiff()

	// Output:
	// -----
	// C a => "{\"V\":\"a1\"}"
	// -----
	// <same>
	// -----
	// U a => "{\"V\":\"a2\"}"
	// -----
	// D a
}
