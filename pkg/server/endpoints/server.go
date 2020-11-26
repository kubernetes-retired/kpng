package endpoints

import (
	"context"
	"runtime/trace"
	"strconv"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"k8s.io/klog"

	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
	"m.cluseau.fr/kube-proxy2/pkg/diffstore"
	"m.cluseau.fr/kube-proxy2/pkg/endpoints"
	"m.cluseau.fr/kube-proxy2/pkg/proxystore"
)

type Server struct {
	Store *proxystore.Store
}

var syncItem = &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{}}

func (s *Server) Watch(res localnetv1.Endpoints_WatchServer) error {
	remote := ""
	{
		ctxPeer, _ := peer.FromContext(res.Context())
		remote = ctxPeer.Addr.String()
	}

	klog.Info("new connection from ", remote)
	defer klog.Info("connection from ", remote, " closed")

	w := NewWatchState(res)

	var rev uint64
	for {
		// wait for client request
		req, err := res.Recv()
		if err != nil {
			return grpc.Errorf(codes.Aborted, "recv error: %v", err)
		}

		klog.V(1).Info("remote ", remote, " requested node ", req.NodeName)

		updated := false
		for !updated {
			rev = s.Store.View(rev, func(tx *proxystore.Tx) {
				w.Update(tx, req.NodeName)
			})

			if err := res.Context().Err(); err != nil {
				return grpc.Errorf(codes.Aborted, "context error: %v", err)
			}

			updated = w.SendDiff()
		}

		// change set sent
		w.Send(syncItem)

		if w.Err != nil {
			return w.Err
		}
	}
}

type WatchState struct {
	res  localnetv1.Endpoints_WatchServer
	Svcs *diffstore.DiffStore
	Seps *diffstore.DiffStore
	pb   *proto.Buffer
	Err  error
}

func NewWatchState(res localnetv1.Endpoints_WatchServer) *WatchState {
	return &WatchState{
		res:  res,
		Svcs: diffstore.New(),
		Seps: diffstore.New(),
		pb:   proto.NewBuffer(make([]byte, 0)),
	}
}

func (w *WatchState) Send(item *localnetv1.OpItem) {
	if w.Err != nil {
		return
	}
	err := w.res.Send(item)
	if err != nil {
		w.Err = grpc.Errorf(codes.Aborted, "send error: %v", err)
	}
}

func (w *WatchState) sendSet(set localnetv1.Set, path string, m proto.Message) {
	w.pb.Reset()
	if err := w.pb.Marshal(m); err != nil {
		panic("protobuf Marshal failed: " + err.Error())
	}

	w.Send(&localnetv1.OpItem{
		Op: &localnetv1.OpItem_Set{
			Set: &localnetv1.Value{
				Ref:   &localnetv1.Ref{Set: set, Path: path},
				Bytes: w.pb.Bytes(),
			},
		},
	})
}

func (w *WatchState) sendDelete(set localnetv1.Set, path string) {
	w.Send(&localnetv1.OpItem{
		Op: &localnetv1.OpItem_Delete{
			Delete: &localnetv1.Ref{Set: set, Path: path},
		},
	})
}

func (w *WatchState) Update(tx *proxystore.Tx, nodeName string) {
	if !tx.AllSynced() {
		return
	}

	ctx, task := trace.NewTask(context.Background(), "WatchState.Update")
	defer task.End()

	// set all new values
	tx.Each(proxystore.Services, func(kv *proxystore.KV) bool {
		key := []byte(kv.Namespace + "/" + kv.Name)

		if trace.IsEnabled() {
			trace.Log(ctx, "service", string(key))
		}
		w.Svcs.Set(key, kv.Service.Hash, kv.Service.Service)

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

			w.Seps.Set(key, ei.Hash, ei.Endpoint)
		}

		return true
	})
}

func (w *WatchState) SendDiff() (updated bool) {
	ctx, task := trace.NewTask(context.Background(), "WatchState.SendDiff")
	defer task.End()

	for _, kv := range w.Svcs.Updated() {
		updated = true
		w.sendSet(localnetv1.Set_ServicesSet, string(kv.Key), kv.Value.(*localnetv1.Service))
		trace.Log(ctx, "service-set", string(kv.Key))
	}
	for _, kv := range w.Seps.Updated() {
		updated = true
		w.sendSet(localnetv1.Set_EndpointsSet, string(kv.Key), kv.Value.(*localnetv1.Endpoint))
		trace.Log(ctx, "endpoint-set", string(kv.Key))
	}
	for _, kv := range w.Seps.Deleted() {
		updated = true
		w.sendDelete(localnetv1.Set_EndpointsSet, string(kv.Key))
		trace.Log(ctx, "endpoint-deleted", string(kv.Key))
	}
	for _, kv := range w.Svcs.Deleted() {
		updated = true
		w.sendDelete(localnetv1.Set_ServicesSet, string(kv.Key))
		trace.Log(ctx, "service-deleted", string(kv.Key))
	}

	w.Svcs.Reset(diffstore.ItemDeleted)
	w.Seps.Reset(diffstore.ItemDeleted)

	return
}
