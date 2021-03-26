package store2diff

import (
	"context"

	"sigs.k8s.io/kpng/pkg/api/localnetv1"
	"sigs.k8s.io/kpng/pkg/proxystore"
	"sigs.k8s.io/kpng/pkg/server/watchstate"
)

var syncItem = &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{}}

type Job struct {
	Store *proxystore.Store
	Sets  []localnetv1.Set
	Sink  Sink
}

type Sink interface {
	watchstate.OpSink

	Wait() error
	Update(tx *proxystore.Tx, w *watchstate.WatchState)
	SendDiff(w *watchstate.WatchState) (updated bool)
}

func (j *Job) Run(ctx context.Context) (err error) {
	w := watchstate.New(j.Sink, j.Sets)

	var (
		rev    uint64
		closed bool
	)

	for {
		if err = ctx.Err(); err != nil {
			// check the context is still active; we expect the wtachstate/sink to fail fast in this case
			return
		}

		// wait
		err = j.Sink.Wait()
		if err != nil {
			return
		}

		updated := false
		for !updated {
			// update the state
			rev, closed = j.Store.View(rev, func(tx *proxystore.Tx) {
				j.Sink.Update(tx, w)
			})

			if closed {
				return
			}

			if w.Err != nil {
				return w.Err
			}

			// send the diff
			updated = j.Sink.SendDiff(w)
		}

		// signal the change set is fully sent
		w.SendSync()

		if w.Err != nil {
			return w.Err
		}
	}
}
