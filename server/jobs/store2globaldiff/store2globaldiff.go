package store2globaldiff

import (
	"context"
	"runtime/trace"
	store2diff2 "sigs.k8s.io/kpng/server/jobs/store2diff"
	localnetv12 "sigs.k8s.io/kpng/server/pkg/api/localnetv1"
	diffstore2 "sigs.k8s.io/kpng/server/pkg/diffstore"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
	watchstate2 "sigs.k8s.io/kpng/server/pkg/server/watchstate"
)

type Sink interface {
	Wait() error
	watchstate2.OpSink
}

type Job struct {
	Store *proxystore2.Store
	Sink  Sink
}

var sets = []localnetv12.Set{
	localnetv12.Set_GlobalNodeInfos,
	localnetv12.Set_GlobalServiceInfos,
	localnetv12.Set_GlobalEndpointInfos,
}

func (j *Job) Run(ctx context.Context) error {
	job := &store2diff2.Job{
		Store: j.Store,
		Sets:  sets,
		Sink:  j,
	}

	return job.Run(ctx)
}

func (j *Job) Wait() (err error) {
	return j.Sink.Wait()
}

func (j *Job) Update(tx *proxystore2.Tx, w *watchstate2.WatchState) {
	if !tx.AllSynced() {
		return
	}

	_, task := trace.NewTask(context.Background(), "GlobalState.Update")
	defer task.End()

	// sync all stores
	for _, set := range sets {
		diff := w.StoreFor(set)
		tx.Each(set, func(kv *proxystore2.KV) bool {
			h := kv.Value.GetHash()
			diff.Set([]byte(kv.Path()), h, kv.Value)
			return true
		})
	}
}

func (_ *Job) SendDiff(w *watchstate2.WatchState) (updated bool) {
	_, task := trace.NewTask(context.Background(), "GlobalState.SendDiff")
	defer task.End()

	count := 0
	count += w.SendUpdates(localnetv12.Set_GlobalNodeInfos)
	count += w.SendUpdates(localnetv12.Set_GlobalServiceInfos)
	count += w.SendUpdates(localnetv12.Set_GlobalEndpointInfos)
	count += w.SendDeletes(localnetv12.Set_GlobalEndpointInfos)
	count += w.SendDeletes(localnetv12.Set_GlobalServiceInfos)
	count += w.SendDeletes(localnetv12.Set_GlobalNodeInfos)

	w.Reset(diffstore2.ItemDeleted)

	return count != 0
}

func (j *Job) Send(op *localnetv12.OpItem) error {
	return j.Sink.Send(op)
}
