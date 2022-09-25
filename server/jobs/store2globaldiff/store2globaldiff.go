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

package store2globaldiff

import (
	"context"
	"runtime/trace"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/lightdiffstore"
	"sigs.k8s.io/kpng/server/jobs/store2diff"
	"sigs.k8s.io/kpng/server/pkg/proxystore"
	"sigs.k8s.io/kpng/server/pkg/server/watchstate"
)

type Sink interface {
	Wait() error
	localnetv1.OpSink
}

type Job struct {
	Store *proxystore.Store
	Sink  Sink
}

var sets = []localnetv1.Set{
	localnetv1.Set_GlobalNodeInfos,
	localnetv1.Set_GlobalServiceInfos,
	localnetv1.Set_GlobalEndpointInfos,
}

func (j *Job) Run(ctx context.Context) error {
	job := &store2diff.Job{
		Store: j.Store,
		Sets:  sets,
		Sink:  j,
	}

	return job.Run(ctx)
}

func (j *Job) Wait() (err error) {
	return j.Sink.Wait()
}

func (j *Job) Update(tx *proxystore.Tx, w *watchstate.WatchState) {
	if !tx.AllSynced() {
		return
	}

	_, task := trace.NewTask(context.Background(), "GlobalState.Update")
	defer task.End()

	// sync all stores
	for _, set := range sets {
		diff := w.StoreFor(set)
		tx.Each(set, func(kv *proxystore.KV) bool {
			h := kv.Value.GetHash()
			diff.Set([]byte(kv.Path()), h, kv.Value)
			return true
		})
	}
}

func (_ *Job) SendDiff(w *watchstate.WatchState) (updated bool) {
	_, task := trace.NewTask(context.Background(), "GlobalState.SendDiff")
	defer task.End()

	count := 0
	count += w.SendUpdates(localnetv1.Set_GlobalNodeInfos)
	count += w.SendUpdates(localnetv1.Set_GlobalServiceInfos)
	count += w.SendUpdates(localnetv1.Set_GlobalEndpointInfos)
	count += w.SendDeletes(localnetv1.Set_GlobalEndpointInfos)
	count += w.SendDeletes(localnetv1.Set_GlobalServiceInfos)
	count += w.SendDeletes(localnetv1.Set_GlobalNodeInfos)

	w.Reset(lightdiffstore.ItemDeleted)

	return count != 0
}

func (j *Job) Send(op *localnetv1.OpItem) error {
	return j.Sink.Send(op)
}
