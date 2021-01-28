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

package localnetv1

import (
	"fmt"
	"testing"
)

func TestInsertString(t *testing.T) {
	a := make([]string, 0)

	expect := func(exp string) {
		t.Helper()
		if s := fmt.Sprint(a); s != exp {
			t.Errorf("expected %s, got %s", exp, s)
		}
	}

	insertString(&a, "b")
	expect("[b]")

	insertString(&a, "d")
	expect("[b d]")

	insertString(&a, "a")
	expect("[a b d]")

	insertString(&a, "c")
	expect("[a b c d]")
}

func ExampleIPSetAdd() {
	s := &IPSet{}

	s.Add("1.1.1.2")
	s.Add("1.1.1.4")
	s.Add("1.1.1.3")
	s.Add("1.1.1.1")
	s.Add("1.1.1.2")

	s.Add("::2")
	s.Add("::4")
	s.Add("::3")
	s.Add("::1")
	s.Add("::2")

	fmt.Println(s.V4)
	fmt.Println(s.V6)

	// Output:
	// [1.1.1.1 1.1.1.2 1.1.1.3 1.1.1.4]
	// [::1 ::2 ::3 ::4]
}
