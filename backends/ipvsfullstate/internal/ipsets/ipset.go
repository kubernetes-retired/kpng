/*
Copyright 2023 The Kubernetes Authors.

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

package ipsets

import (
	"fmt"
	"k8s.io/klog/v2"
)

type Set struct {
	IPSet
	// handle is the util ipset interface handle.
	handle Interface

	refCountOfSvc int
}

// newIPSet initialize a new Set struct
func newIPSet(handle Interface, name string, setType SetType, hashFamily ProtocolFamily, comment string) *Set {
	set := &Set{
		IPSet: IPSet{
			Name:       name,
			SetType:    setType,
			HashFamily: hashFamily,
			Comment:    comment,
		},
		handle: handle,
	}
	return set
}

func (set *Set) validateEntry(entry *Entry) bool {
	return entry.Validate(set)
}

func (set *Set) GetComment() string {
	return fmt.Sprintf("\"%s\"", set.Comment)
}

func (set *Set) GetName() string {
	return set.IPSet.Name
}

func (set *Set) addEntry(entry *Entry) error {
	return set.handle.AddEntry(entry.String(), &set.IPSet, true)
}

func (set *Set) delEntry(entry *Entry) error {
	return set.handle.DelEntry(entry.String(), set.GetName())
}

func ensureIPSet(set *Set) error {
	if err := set.handle.CreateSet(&set.IPSet, true); err != nil {
		klog.Errorf("Failed to ensure ip set %v exist, error: %v", set, err)
		return err
	}
	return nil
}
