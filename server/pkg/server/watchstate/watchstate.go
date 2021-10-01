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

package watchstate

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	localnetv12 "sigs.k8s.io/kpng/api/localnetv1"
	diffstore2 "sigs.k8s.io/kpng/server/pkg/diffstore"
)

type OpSink interface {
	Send(op *localnetv12.OpItem) error
}

type WatchState struct {
	res   OpSink
	sets  []localnetv12.Set
	diffs []*diffstore2.DiffStore
	pb    *proto.Buffer
	Err   error
}

func New(res OpSink, sets []localnetv12.Set) *WatchState {
	diffs := make([]*diffstore2.DiffStore, len(sets))
	for i := range sets {
		diffs[i] = diffstore2.New()
	}

	return &WatchState{
		res:   res,
		sets:  sets,
		diffs: diffs,
		pb:    proto.NewBuffer(make([]byte, 0)),
	}
}

func (w *WatchState) StoreFor(set localnetv12.Set) *diffstore2.DiffStore {
	for i, s := range w.sets {
		if s == set {
			return w.diffs[i]
		}
	}
	panic(fmt.Errorf("not watching set %v", set))
}

func (w *WatchState) SendUpdates(set localnetv12.Set) (count int) {
	if w.Err != nil {
		return
	}

	store := w.StoreFor(set)

	updated := store.Updated()

	for _, kv := range updated {
		w.sendSet(set, string(kv.Key), kv.Value.(proto.Message))
	}

	return len(updated)
}

func (w *WatchState) SendDeletes(set localnetv12.Set) (count int) {
	if w.Err != nil {
		return
	}

	store := w.StoreFor(set)

	deleted := store.Deleted()

	for _, kv := range deleted {
		w.sendDelete(set, string(kv.Key))
	}

	return len(deleted)
}

func (w *WatchState) send(item *localnetv12.OpItem) {
	if w.Err != nil {
		return
	}
	err := w.res.Send(item)
	if err != nil {
		w.Err = grpc.Errorf(codes.Aborted, "send error: %v", err)
	}
}

func (w *WatchState) sendSet(set localnetv12.Set, path string, m proto.Message) {
	w.pb.Reset()
	if err := w.pb.Marshal(m); err != nil {
		panic("protobuf Marshal failed: " + err.Error())
	}

	w.send(&localnetv12.OpItem{
		Op: &localnetv12.OpItem_Set{
			Set: &localnetv12.Value{
				Ref:   &localnetv12.Ref{Set: set, Path: path},
				Bytes: w.pb.Bytes(),
			},
		},
	})
}

func (w *WatchState) sendDelete(set localnetv12.Set, path string) {
	w.send(&localnetv12.OpItem{
		Op: &localnetv12.OpItem_Delete{
			Delete: &localnetv12.Ref{Set: set, Path: path},
		},
	})
}

func (w *WatchState) Reset(state diffstore2.ItemState) {
	for _, s := range w.diffs {
		s.Reset(state)
	}
}

var syncItem = &localnetv12.OpItem{Op: &localnetv12.OpItem_Sync{}}

func (w *WatchState) SendSync() {
	w.send(syncItem)
}

var resetItem = &localnetv12.OpItem{Op: &localnetv12.OpItem_Reset_{}}

func (w *WatchState) SendReset() {
	w.send(resetItem)
}
