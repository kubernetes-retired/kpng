package store2localdiff

import (
	"context"
	"runtime/trace"
	"strconv"

	"github.com/cespare/xxhash"
	"github.com/golang/protobuf/proto"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/server/jobs/store2diff"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/pkg/diffstore"
	"sigs.k8s.io/kpng/server/pkg/endpoints"
	"sigs.k8s.io/kpng/server/pkg/proxystore"
	"sigs.k8s.io/kpng/server/pkg/server/watchstate"
)

type Job struct {
	Store *proxystore.Store
	Sink  localsink.Sink
}

func (j *Job) Run(ctx context.Context) error {
	run := &jobRun{
		Sink: j.Sink,
		buf:  proto.NewBuffer(make([]byte, 0, 256)),
	}

	job := &store2diff.Job{
		Store: j.Store,
		Sets: []localnetv1.Set{
			localnetv1.Set_ServicesSet,
			localnetv1.Set_EndpointsSet,
		},
		Sink: run,
	}

	j.Sink.Setup()

	return job.Run(ctx)
}

type jobRun struct {
	localsink.Sink
	nodeName string
	buf      *proto.Buffer
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

	svcs := w.StoreFor(localnetv1.Set_ServicesSet)
	seps := w.StoreFor(localnetv1.Set_EndpointsSet)

	// set all new values
	tx.Each(proxystore.Services, func(kv *proxystore.KV) bool {
		key := []byte(kv.Namespace + "/" + kv.Name)

		if trace.IsEnabled() {
			trace.Log(ctx, "service", string(key))
		}
		svcs.Set(key, kv.Service.Hash, kv.Service.Service)

		// filter endpoints for this node
		endpointInfos := endpoints.ForNode(tx, kv.Service, nodeName)

		for _, ei := range endpointInfos {
			// hash only the endpoint
			s.buf.Marshal(ei.Endpoint)
			hash := xxhash.Sum64(s.buf.Bytes())
			s.buf.Reset()

			// key is service key + endpoint hash (64 bits, in hex)
			key := append(make([]byte, 0, len(key)+1+64/8*2), key...)
			key = append(key, '/')
			key = strconv.AppendUint(key, hash, 16)

			if trace.IsEnabled() {
				trace.Log(ctx, "endpoint", string(key))
			}

			seps.Set(key, hash, ei.Endpoint)
		}

		return true
	})
}

func (_ *jobRun) SendDiff(w *watchstate.WatchState) (updated bool) {
	_, task := trace.NewTask(context.Background(), "LocalState.SendDiff")
	defer task.End()

	count := 0
	count += w.SendUpdates(localnetv1.Set_ServicesSet)
	count += w.SendUpdates(localnetv1.Set_EndpointsSet)
	count += w.SendDeletes(localnetv1.Set_EndpointsSet)
	count += w.SendDeletes(localnetv1.Set_ServicesSet)

	w.Reset(diffstore.ItemDeleted)

	return count != 0
}
