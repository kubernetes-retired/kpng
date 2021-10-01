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

package fullstate

import "testing"

func TestLess(t *testing.T) {
	kvs := []kv{
		kv{"ns/aaa", nil},
		kv{"ns/aaa/0", nil},
		kv{"ns/aaa-udp", nil},
		kv{"ns/aaa-udp/0", nil},
		kv{"ns/bbb", nil},
		kv{"ns/ccc/0", nil},
	}

	for i := 1; i < len(kvs); i++ {
		if !kvs[i-1].Less(kvs[i]) {
			t.Errorf("expected [%d] %s < [%d] %s", i-1, kvs[i-1].Path, i, kvs[i].Path)
		}
		if kvs[i].Less(kvs[i-1]) {
			t.Errorf("expected [%d] %s > [%d] %s", i, kvs[i].Path, i-1, kvs[i-1].Path)
		}
	}
}

func TestEqual(t *testing.T) {
	k1 := kv{"ns/aaa", nil}
	k2 := kv{"ns/aaa", nil}

	if k1.Less(k2) {
		t.Error("k1 < k2")
	}
	if k2.Less(k1) {
		t.Error("k2 < k1")
	}
}
