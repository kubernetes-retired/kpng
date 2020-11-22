package endpoints

import (
	"strconv"

	"github.com/cespare/xxhash"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"k8s.io/klog"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/diffstore"
	"github.com/mcluseau/kube-proxy2/pkg/endpoints"
	"github.com/mcluseau/kube-proxy2/pkg/proxystore"
)

type Server struct {
	Store *proxystore.Store
}

var syncItem = &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{}}

func (s *Server) Watch(res localnetv1.Endpoints_WatchServer) error {
	remote, _ := peer.FromContext(res.Context())
	klog.Info("new connection from ", remote.Addr)
	defer klog.Info("connection from ", remote.Addr, " closed")

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
				klog.V(1).Info("sending update to ", remote)
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
		pb:   proto.NewBuffer(nil),
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

func (w *WatchState) HashOf(m proto.Message) uint64 {
	w.pb.Reset()
	if err := w.pb.Marshal(m); err != nil {
		panic("protobuf Marshal failed: " + err.Error())
	}

	return xxhash.Sum64(w.pb.Bytes())
}

func (w *WatchState) Update(tx *proxystore.Tx, nodeName string) {
	if !tx.AllSynced() {
		return
	}

	// set all new values
	tx.Each(proxystore.Services, func(kv *proxystore.KV) bool {
		key := []byte(kv.Namespace + "/" + kv.Name)

		w.Svcs.Set(key, w.HashOf(kv.Service), kv.Service)

		// filter endpoints for this node
		endpoints := endpoints.ForNode(tx, kv.Service, kv.TopologyKeys, nodeName)

		for _, ep := range endpoints {
			// key is service key + endpoint hash (64 bits, in hex)
			key := append(make([]byte, 0, len(key)+1+64/8*2), key...)
			key = append(key, '/')

			h := w.HashOf(ep)
			key = strconv.AppendUint(key, h, 16)

			w.Seps.Set(key, h, ep)
		}

		return true
	})
}

func (w *WatchState) SendDiff() (updated bool) {
	for _, kv := range w.Svcs.Updated() {
		updated = true
		w.sendSet(localnetv1.Set_ServicesSet, string(kv.Key), kv.Value.(*localnetv1.Service))
	}
	for _, kv := range w.Seps.Updated() {
		updated = true
		w.sendSet(localnetv1.Set_EndpointsSet, string(kv.Key), kv.Value.(*localnetv1.Endpoint))
	}
	for _, kv := range w.Seps.Deleted() {
		updated = true
		w.sendDelete(localnetv1.Set_EndpointsSet, string(kv.Key))
	}
	for _, kv := range w.Svcs.Deleted() {
		updated = true
		w.sendDelete(localnetv1.Set_ServicesSet, string(kv.Key))
	}

	w.Svcs.Reset(diffstore.ItemDeleted)
	w.Seps.Reset(diffstore.ItemDeleted)

	return
}
