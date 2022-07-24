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

package serviceevents

// TODO can be extracted as a generic package if needed

type Diff struct {
	SameKey func(prevIdx, currIdx int) bool
	Added   func(currIdx int)
	Updated func(prevIdx, currIdx int)
	Deleted func(prevIdx int)
}

func (d Diff) SlicesLen(prevLen, currLen int) {
prevLoop:
	for i := 0; i < prevLen; i++ {
		for j := 0; j < currLen; j++ {
			if d.SameKey(i, j) {
				d.Updated(i, j)
				continue prevLoop
			}
		}

		// previous value not found in current values
		d.Deleted(i)
	}

currLoop:
	for j := 0; j < currLen; j++ {
		for i := 0; i < prevLen; i++ {
			if d.SameKey(i, j) {
				continue currLoop
			}
		}

		// current value not found in previous values
		d.Added(j)
	}
}
