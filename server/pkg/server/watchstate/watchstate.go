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

	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/lightdiffstore"
	"sigs.k8s.io/kpng/server/pkg/metrics"
)

type WatchState struct {
	res   localv1.OpSink
	sets  []localv1.Set
	diffs []*lightdiffstore.DiffStore
	Err   error
}

func New(res localv1.OpSink, sets []localv1.Set) *WatchState {
	diffs := make([]*lightdiffstore.DiffStore, len(sets))
	for i := range sets {
		diffs[i] = lightdiffstore.New()
	}

	return &WatchState{
		res:   res,
		sets:  sets,
		diffs: diffs,
	}
}

func (w *WatchState) StoreFor(set localv1.Set) *lightdiffstore.DiffStore {
	return w.StoreForN(set, 0)
}

func (w *WatchState) StoreForN(set localv1.Set, setN int) *lightdiffstore.DiffStore {
	n := 0
	for i, s := range w.sets {
		if s == set {
			if setN == n {
				return w.diffs[i]
			} else {
				n++
			}
		}
	}
	panic(fmt.Errorf("not watching set %v[%d]", set, setN))
}

func (w *WatchState) SendUpdates(set localv1.Set) (count int) {
	return w.SendUpdatesN(set, 0)
}

func (w *WatchState) SendUpdatesN(set localv1.Set, setN int) (count int) {
	if w.Err != nil {
		return
	}

	store := w.StoreForN(set, setN)

	updated := store.Updated()

	for _, kv := range updated {
		w.sendSet(set, string(kv.Key), kv.Value.(proto.Message))
	}

	return len(updated)
}

func (w *WatchState) SendDeletes(set localv1.Set) (count int) {
	return w.SendDeletesN(set, 0)
}

func (w *WatchState) SendDeletesN(set localv1.Set, setN int) (count int) {
	if w.Err != nil {
		return
	}

	store := w.StoreForN(set, setN)

	deleted := store.Deleted()

	for _, kv := range deleted {
		w.sendDelete(set, string(kv.Key))
	}

	return len(deleted)
}

func (w *WatchState) send(item *localv1.OpItem) {
	if w.Err != nil {
		return
	}
	metrics.Kpng_node_local_events.Inc()
	err := w.res.Send(item)
	if err != nil {
		w.Err = grpc.Errorf(codes.Aborted, "send error: %v", err)
	}
}

func (w *WatchState) sendSet(set localv1.Set, path string, m proto.Message) {
	message, err := proto.Marshal(m)
	if err != nil {
		panic("protobuf Marshal failed: " + err.Error())
	}

	w.send(&localv1.OpItem{
		Op: &localv1.OpItem_Set{
			Set: &localv1.Value{
				Ref:   &localv1.Ref{Set: set, Path: path},
				Bytes: message,
			},
		},
	})
}

func (w *WatchState) sendDelete(set localv1.Set, path string) {
	w.send(&localv1.OpItem{
		Op: &localv1.OpItem_Delete{
			Delete: &localv1.Ref{Set: set, Path: path},
		},
	})
}

func (w *WatchState) Reset(state lightdiffstore.ItemState) {
	for _, s := range w.diffs {
		s.Reset(state)
	}
}

var syncItem = &localv1.OpItem{Op: &localv1.OpItem_Sync{}}

func (w *WatchState) SendSync() {
	w.send(syncItem)
}

var resetItem = &localv1.OpItem{Op: &localv1.OpItem_Reset_{}}

func (w *WatchState) SendReset() {
	w.send(resetItem)
}
