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
)

func (store *Store[K, V]) printDiff() {
	fmt.Println("-----")
	hasChanges := false

	for _, i := range store.Changed() {
		hasChanges = true

		s := "U"
		if i.Created() {
			s = "C"
		}
		fmt.Printf("%s %v => %q\n", s, i.Key(), fmt.Sprint(i.Value()))
	}

	for _, i := range store.Deleted() {
		hasChanges = true

		fmt.Printf("D %v\n", i.Key())
	}

	if !hasChanges {
		fmt.Println("<same>")
	}
}
