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

package filterreset

import (
	"github.com/cespare/xxhash"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink"
)

type Sink struct {
	sink      localsink.Sink
	filtering bool
	memory    map[string]memItem
	seen      map[string]bool
}

type memItem struct {
	set  localnetv1.Set
	hash uint64
}

var _ localsink.Sink = &Sink{}

func New(sink localsink.Sink) *Sink {
	return &Sink{
		sink:   sink,
		memory: map[string]memItem{},
	}
}

func (s *Sink) Setup() { s.sink.Setup() }

func (s *Sink) WaitRequest() (nodeName string, err error) {
	return s.sink.WaitRequest()
}

func (s *Sink) Reset() {
	s.filtering = true
	s.seen = make(map[string]bool, len(s.memory))
}

func (s *Sink) Send(op *localnetv1.OpItem) (err error) {
	switch v := op.Op; v.(type) {
	case *localnetv1.OpItem_Set:
		set := op.GetSet()
		path := set.Ref.Path

		if s.filtering {
			s.seen[path] = true
		}

		h := xxhash.Sum64(set.Bytes)

		if s.memory[path].hash == h {
			return // updated to the same value => filtered
		}

		s.memory[path] = memItem{
			set:  set.Ref.Set,
			hash: h,
		}

		return s.sink.Send(op)

	case *localnetv1.OpItem_Delete:
		del := op.GetDelete()

		if _, exists := s.memory[del.Path]; exists {
			delete(s.memory, del.Path)
			return s.sink.Send(op)
		}

		return nil

	case *localnetv1.OpItem_Sync:
		if s.filtering {
			toDelete := make([]string, 0)
			for path, mem := range s.memory {
				if !s.seen[path] {
					toDelete = append(toDelete, path)

					err = s.sink.Send(&localnetv1.OpItem{
						Op: &localnetv1.OpItem_Delete{
							Delete: &localnetv1.Ref{
								Set:  mem.set,
								Path: path,
							},
						},
					})

					if err != nil {
						return
					}
				}
			}

			for _, path := range toDelete {
				delete(s.memory, path)
			}

			s.filtering = false
			s.seen = nil
		}

		return s.sink.Send(op)

	default:
		return s.sink.Send(op)
	}
}
