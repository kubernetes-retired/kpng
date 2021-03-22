package store2localdiff

import (
	"context"
	"runtime/trace"
	"strconv"

	"github.com/spf13/pflag"

	"m.cluseau.fr/kpng/jobs/store2diff"
	"m.cluseau.fr/kpng/pkg/api/localnetv1"
	"m.cluseau.fr/kpng/pkg/diffstore"
	"m.cluseau.fr/kpng/pkg/endpoints"
	"m.cluseau.fr/kpng/pkg/proxystore"
	"m.cluseau.fr/kpng/pkg/server/watchstate"
)

type Config struct {
	NodeName string
}

func (c *Config) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&c.NodeName, "node-name", "", "Node name override")
}

type Sink interface {
	// WaitRequest waits for the next diff request, returning the requested node name. If an error is returned, it will cancel the job.
	WaitRequest() (nodeName string, err error)

	watchstate.OpSink
}

type Job struct {
	Store *proxystore.Store
	Sink  Sink
}

func (j *Job) Run() error {
	run := &jobRun{Sink: j.Sink}

	job := &store2diff.Job{
		Store: j.Store,
		Sets: []localnetv1.Set{
			localnetv1.Set_ServicesSet,
			localnetv1.Set_EndpointsSet,
		},
		Sink: run,
	}

	return job.Run()
}

type jobRun struct {
	Sink
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
			// key is service key + endpoint hash (64 bits, in hex)
			key := append(make([]byte, 0, len(key)+1+64/8*2), key...)
			key = append(key, '/')
			key = strconv.AppendUint(key, ei.Hash, 16)

			if trace.IsEnabled() {
				trace.Log(ctx, "endpoint", string(key))
			}

			seps.Set(key, ei.Hash, ei.Endpoint)
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
