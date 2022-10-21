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

package store2diff

import (
	"context"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/server/pkg/server/watchstate"
	"sigs.k8s.io/kpng/server/proxystore"
)

type Job struct {
	Store *proxystore.Store
	Sets  []localnetv1.Set
	Sink  Sink
}

type Sink interface {
	localnetv1.OpSink

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

		if rev == 0 {
			w.SendReset()
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
