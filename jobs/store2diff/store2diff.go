package store2diff

import (
	"m.cluseau.fr/kpng/pkg/api/localnetv1"
	"m.cluseau.fr/kpng/pkg/proxystore"
	"m.cluseau.fr/kpng/pkg/server/watchstate"
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

func (j *Job) Run() error {
	w := watchstate.New(j.Sink, j.Sets)

	var rev uint64
	for {
		// wait
		err := j.Sink.Wait()
		if err != nil {
			return err
		}

		updated := false
		for !updated {
			// update the state
			rev = j.Store.View(rev, func(tx *proxystore.Tx) {
				j.Sink.Update(tx, w)
			})

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
