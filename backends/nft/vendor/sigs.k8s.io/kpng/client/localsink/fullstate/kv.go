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

import (
	"github.com/google/btree"
	"google.golang.org/protobuf/proto"
)

type kv struct {
	Path  string
	Value proto.Message
}

func (kv1 kv) Less(i btree.Item) bool {
	// path separator aware less

	kv2 := i.(kv)

	p1 := kv1.Path
	p2 := kv2.Path

	minLen := len(p1)
	lessIfShort := true

	if l := len(p2); l <= minLen {
		minLen = l
		lessIfShort = false
	}

	for i := 0; i < minLen; i++ {
		r1, r2 := p1[i], p2[i]

		if r1 == r2 {
			continue
		}

		if r1 == '/' {
			return true
		} else if r2 == '/' {
			return false
		} else {
			return r1 < r2
		}
	}

	return lessIfShort
}
