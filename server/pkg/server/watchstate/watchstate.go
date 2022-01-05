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

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/pkg/diffstore"
)

type WatchState struct {
	res   localnetv1.OpSink
	sets  []localnetv1.Set
	diffs []*diffstore.DiffStore
	Err   error
}

func New(res localnetv1.OpSink, sets []localnetv1.Set) *WatchState {
	diffs := make([]*diffstore.DiffStore, len(sets))
	for i := range sets {
		diffs[i] = diffstore.New()
	}

	return &WatchState{
		res:   res,
		sets:  sets,
		diffs: diffs,
	}
}

func (w *WatchState) StoreFor(set localnetv1.Set) *diffstore.DiffStore {
	for i, s := range w.sets {
		if s == set {
			return w.diffs[i]
		}
	}
	panic(fmt.Errorf("not watching set %v", set))
}

func (w *WatchState) SendUpdates(set localnetv1.Set) (count int) {
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

func (w *WatchState) SendDeletes(set localnetv1.Set) (count int) {
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

func (w *WatchState) send(item *localnetv1.OpItem) {
	if w.Err != nil {
		return
	}
	err := w.res.Send(item)
	if err != nil {
		w.Err = grpc.Errorf(codes.Aborted, "send error: %v", err)
	}
}

func (w *WatchState) sendSet(set localnetv1.Set, path string, m proto.Message) {
	message, err := proto.Marshal(m)
	if err != nil {
		panic("protobuf Marshal failed: " + err.Error())
	}

	w.send(&localnetv1.OpItem{
		Op: &localnetv1.OpItem_Set{
			Set: &localnetv1.Value{
				Ref:   &localnetv1.Ref{Set: set, Path: path},
				Bytes: message,
			},
		},
	})
}

func (w *WatchState) sendDelete(set localnetv1.Set, path string) {
	w.send(&localnetv1.OpItem{
		Op: &localnetv1.OpItem_Delete{
			Delete: &localnetv1.Ref{Set: set, Path: path},
		},
	})
}

func (w *WatchState) Reset(state diffstore.ItemState) {
	for _, s := range w.diffs {
		s.Reset(state)
	}
}

var syncItem = &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{}}

func (w *WatchState) SendSync() {
	w.send(syncItem)
}

var resetItem = &localnetv1.OpItem{Op: &localnetv1.OpItem_Reset_{}}

func (w *WatchState) SendReset() {
	w.send(resetItem)
}
