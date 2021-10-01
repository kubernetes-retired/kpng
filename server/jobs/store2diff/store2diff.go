package store2diff

import (
	"context"

	localnetv12 "sigs.k8s.io/kpng/api/localnetv1"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
	watchstate2 "sigs.k8s.io/kpng/server/pkg/server/watchstate"
)

type Job struct {
	Store *proxystore2.Store
	Sets  []localnetv12.Set
	Sink  Sink
}

type Sink interface {
	watchstate2.OpSink

	Wait() error
	Update(tx *proxystore2.Tx, w *watchstate2.WatchState)
	SendDiff(w *watchstate2.WatchState) (updated bool)
}

func (j *Job) Run(ctx context.Context) (err error) {
	w := watchstate2.New(j.Sink, j.Sets)

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

		if rev == 0 {
			w.SendReset()
		}

		updated := false
		for !updated {
			// update the state
			rev, closed = j.Store.View(rev, func(tx *proxystore2.Tx) {
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
