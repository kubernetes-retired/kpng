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

package lightdiffstore

import (
	"fmt"

	"github.com/cespare/xxhash"
)

func ExampleDiffStore() {
	s := New()

	set := func(k, v string) {
		fmt.Printf("set %v to %s\n", k, v)
		s.Set([]byte(k), xxhash.Sum64([]byte(v)), v)
	}

	set("a", "alice")
	set("b", "bob")

	fmt.Println("-> updated:", s.Updated(), "deleted:", s.Deleted())

	// --------------------------------------------------------------------------
	fmt.Println()
	s.Reset(ItemUnchanged)

	set("a", "alice2")

	fmt.Println("-> updated:", s.Updated(), "deleted:", s.Deleted())

	// --------------------------------------------------------------------------
	fmt.Println()
	s.Reset(ItemDeleted)

	set("a", "alice3")

	fmt.Println("-> updated:", s.Updated(), "deleted:", s.Deleted())

	// --------------------------------------------------------------------------
	fmt.Println()
	s.Reset(ItemDeleted)

	set("a", "alice3")
	set("b", "bob")

	fmt.Println("-> updated:", s.Updated(), "deleted:", s.Deleted())

	// double reset will remove all entries (and should not crash)
	s.Reset(ItemDeleted)
	s.Reset(ItemDeleted)

	fmt.Println("tree size after double reset:", s.tree.Len())

	// Output:
	// set a to alice
	// set b to bob
	// -> updated: [{a => alice} {b => bob}] deleted: []
	//
	// set a to alice2
	// -> updated: [{a => alice2}] deleted: []
	//
	// set a to alice3
	// -> updated: [{a => alice3}] deleted: [{b => bob}]
	//
	// set a to alice3
	// set b to bob
	// -> updated: [{b => bob}] deleted: []
	// tree size after double reset: 0
}

func ExampleDiffPortMapping() {
	s := New()

	set := func(ports ...int) {
		s.Reset(ItemDeleted)

		for _, port := range ports {
			key := fmt.Sprintf("%s/%s:%d", "my-ns", "my-svc", port)
			s.Set([]byte(key), 0, port)
		}

		fmt.Println("updated:", s.Updated(), "deleted:", s.Deleted())
	}
	set(80)
	set(80, 81)
	set(443)

	// Output:
	// updated: [{my-ns/my-svc:80 => 80}] deleted: []
	// updated: [{my-ns/my-svc:81 => 81}] deleted: []
	// updated: [{my-ns/my-svc:443 => 443}] deleted: [{my-ns/my-svc:81 => 81} {my-ns/my-svc:80 => 80}]
}
