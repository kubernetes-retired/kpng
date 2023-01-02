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

package store2localdiff

import (
	"context"
	"runtime/trace"
	"strconv"

	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/lightdiffstore"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/server/jobs/store2diff"
	"sigs.k8s.io/kpng/server/pkg/endpoints"
	"sigs.k8s.io/kpng/server/pkg/server/watchstate"
	"sigs.k8s.io/kpng/server/proxystore"
	"sigs.k8s.io/kpng/server/serde"
)

type Job struct {
	Store *proxystore.Store
	Sink  localsink.Sink
}

func (j *Job) Run(ctx context.Context) error {
	run := &jobRun{
		Sink: j.Sink,
	}

	job := &store2diff.Job{
		Store: j.Store,
		Sets: []localv1.Set{
			// Each one of these Sets will be used in a diffstore below.
			localv1.Set_ServicesSet,  // setN 0
			localv1.Set_EndpointsSet, // setN 0
			localv1.Set_EndpointsSet, // setN 1
			// 2nd endpoints set for endpoints which do not have a corresponding pod name
		},
		Sink: run,
	}

	j.Sink.Setup()

	return job.Run(ctx)
}

type jobRun struct {
	localsink.Sink
	nodeName string
}

func (s *jobRun) Wait() (err error) {
	s.nodeName, err = s.WaitRequest()
	return
}

func (s *jobRun) Update(tx *proxystore.Tx, w *watchstate.WatchState) {
	if !tx.AllSynced() {
		return
	}

	nodeName := s.nodeName

	ctx, task := trace.NewTask(context.Background(), "LocalState.Update")
	defer task.End()

	// Lookup the existing diffstores here, so we can update them...
	// We have 2 different endpoint sets: named endpoints
	// and unnamed "anonymous" endpoints.
	svcs := w.StoreForN(localv1.Set_ServicesSet, 0)
	seps := w.StoreForN(localv1.Set_EndpointsSet, 0)
	sepsAnonymous := w.StoreForN(localv1.Set_EndpointsSet, 1)

	// set all new values
	tx.Each(proxystore.Services, func(kv *proxystore.BTreeItem) bool {
		key := []byte(kv.Namespace + "/" + kv.Name)

		if trace.IsEnabled() {
			trace.Log(ctx, "service", string(key))
		}
		svcs.Set(key, kv.Service.Hash, kv.Service.Service)

		// iterate through ONLY the endpoints which are valid for
		// this node to loadbalance to (i.e. in cases of
		// topology constraints or trafficPolicy=Local,
		// some endpoints may not be available for
		// node to route to).
		for _, ei := range endpoints.ForNode(tx, kv.Service, nodeName) {
			// endpoints are not hashed, so hash, but hash ONLY the endpoint.
			// to avoid false diff triggering in cases where endpoint metadata
			// not relevant for "local" decision making (i.e. an endpoint
			// annotation or label that is non-consequential).
			hash := serde.Hash(ei.Endpoint)

			var epKey []byte
			// set is a localv1.Set
			var set *lightdiffstore.DiffStore

			if ei.PodName == "" {
				set = sepsAnonymous
				// key is service key + endpoint hash (64 bits, in hex)
				epKey = append(make([]byte, 0, len(key)+1+64/8*2), key...)
				epKey = append(epKey, '/')
				epKey = strconv.AppendUint(epKey, hash, 16)
			} else {
				set = seps
				// key is service key + podName
				epKey = append(make([]byte, 0, len(key)+1+len(ei.PodName)), key...)
				epKey = append(epKey, '/')
				epKey = append(epKey, []byte(ei.PodName)...)
			}

			if trace.IsEnabled() {
				trace.Log(ctx, "endpoint", string(epKey))
			}

			// Insert or update this key in the diffstore
			set.Set(epKey, hash, ei.Endpoint)
		}

		return true
	})
}

// SendDiff implements the store2diff interface.  Called whenever
// the store2diff implementation recieves an updated from the underlying store.
// See the store2diff impl for this logic.
func (*jobRun) SendDiff(w *watchstate.WatchState) (updated bool) {
	_, task := trace.NewTask(context.Background(), "LocalState.SendDiff")
	defer task.End()

	count := 0

	// Create any service first, to avoid orphan endpoints being sent.
	count += w.SendUpdates(localv1.Set_ServicesSet)

	// Now delete anonymous endpoints (n=1, see comments above)
	count += w.SendDeletesN(localv1.Set_EndpointsSet, 1)

	// Now send updates for regular endpoints
	count += w.SendUpdates(localv1.Set_EndpointsSet)
	// And delete the regular endpoints if any
	count += w.SendDeletes(localv1.Set_EndpointsSet)

	// New anonymous endpoints added
	count += w.SendUpdatesN(localv1.Set_EndpointsSet, 1)

	// last, we delete any services , so that no endpoints are orphaned
	// prematurely.
	count += w.SendDeletes(localv1.Set_ServicesSet)

	// Tell the diffstore that every item is now in the previous
	// window, so the store is empty.
	w.Reset(lightdiffstore.ItemDeleted)

	return count != 0
}
