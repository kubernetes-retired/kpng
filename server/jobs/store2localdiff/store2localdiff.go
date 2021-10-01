package store2localdiff

import (
	"context"
	"runtime/trace"
	"strconv"

	"github.com/cespare/xxhash"
	"github.com/golang/protobuf/proto"

	localnetv12 "sigs.k8s.io/kpng/api/localnetv1"
	store2diff2 "sigs.k8s.io/kpng/server/jobs/store2diff"
	"sigs.k8s.io/kpng/client/localsink"
	diffstore2 "sigs.k8s.io/kpng/server/pkg/diffstore"
	endpoints2 "sigs.k8s.io/kpng/server/pkg/endpoints"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
	watchstate2 "sigs.k8s.io/kpng/server/pkg/server/watchstate"
)

type Job struct {
	Store *proxystore2.Store
	Sink  localsink.Sink
}

func (j *Job) Run(ctx context.Context) error {
	run := &jobRun{
		Sink: j.Sink,
		buf:  proto.NewBuffer(make([]byte, 0, 256)),
	}

	job := &store2diff2.Job{
		Store: j.Store,
		Sets: []localnetv12.Set{
			localnetv12.Set_ServicesSet,
			localnetv12.Set_EndpointsSet,
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

func (s *jobRun) Update(tx *proxystore2.Tx, w *watchstate2.WatchState) {
	if !tx.AllSynced() {
		return
	}

	nodeName := s.nodeName

	ctx, task := trace.NewTask(context.Background(), "LocalState.Update")
	defer task.End()

	svcs := w.StoreFor(localnetv12.Set_ServicesSet)
	seps := w.StoreFor(localnetv12.Set_EndpointsSet)

	// set all new values
	tx.Each(proxystore2.Services, func(kv *proxystore2.KV) bool {
		key := []byte(kv.Namespace + "/" + kv.Name)

		if trace.IsEnabled() {
			trace.Log(ctx, "service", string(key))
		}
		svcs.Set(key, kv.Service.Hash, kv.Service.Service)

		// filter endpoints for this node
		endpointInfos := endpoints2.ForNode(tx, kv.Service, nodeName)

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

func (_ *jobRun) SendDiff(w *watchstate2.WatchState) (updated bool) {
	_, task := trace.NewTask(context.Background(), "LocalState.SendDiff")
	defer task.End()

	count := 0
	count += w.SendUpdates(localnetv12.Set_ServicesSet)
	count += w.SendUpdates(localnetv12.Set_EndpointsSet)
	count += w.SendDeletes(localnetv12.Set_EndpointsSet)
	count += w.SendDeletes(localnetv12.Set_ServicesSet)

	w.Reset(diffstore2.ItemDeleted)

	return count != 0
}
