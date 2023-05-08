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

	localv1 "sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/localsink"
)

type ServiceEndpoints struct {
	Service   *localv1.Service
	Endpoints []*localv1.Endpoint
}

type Callback func(item <-chan *ServiceEndpoints)
type Setup func()

type Sink struct {
	Config    *localsink.Config
	Callback  Callback
	SetupFunc Setup

	data *btree.BTree
}

func New(config *localsink.Config) *Sink {
	return &Sink{
		Config: config,
		data:   btree.New(2),
	}
}

var _ localsink.Sink = &Sink{}

// ArrayCallback wraps a array callback
func ArrayCallback(callback func([]*ServiceEndpoints)) Callback {
	items := make([]*ServiceEndpoints, 0)

	return func(ch <-chan *ServiceEndpoints) {
		items = items[:0]

		for seps := range ch {
			items = append(items, seps)
		}

		callback(items)

		return
	}
}

func (s *Sink) Setup() {
	if s.SetupFunc != nil {
		s.SetupFunc()
	}
}

func (s *Sink) WaitRequest() (nodeName string, err error) {
	return s.Config.NodeName, nil
}

func (s *Sink) Reset() {
	s.data.Clear(false)
}

func (s *Sink) Send(op *localv1.OpItem) (err error) {
	switch v := op.Op; v.(type) {
	case *localv1.OpItem_Set:
		set := op.GetSet()

		var v proto.Message
		switch set.Ref.Set {
		case localv1.Set_ServicesSet:
			v = &localv1.Service{}
		case localv1.Set_EndpointsSet:
			v = &localv1.Endpoint{}

		default:
			return
		}

		err = proto.Unmarshal(set.Bytes, v)
		if err != nil {
			return
		}

		s.data.ReplaceOrInsert(kv{set.Ref.Path, v})

	case *localv1.OpItem_Delete:
		s.data.Delete(kv{Path: op.GetDelete().Path})

	case *localv1.OpItem_Sync:
		results := make(chan *ServiceEndpoints)

		go func() {
			defer close(results)

			var seps *ServiceEndpoints

			s.data.Ascend(func(i btree.Item) bool {
				switch v := i.(kv).Value.(type) {
				case *localv1.Service:
					if seps != nil {
						results <- seps
					}

					seps = &ServiceEndpoints{Service: v}
				case *localv1.Endpoint:
					seps.Endpoints = append(seps.Endpoints, v)
				}

				return true
			})

			if seps != nil {
				results <- seps
			}
		}()

		s.Callback(results)
	}

	return
}
