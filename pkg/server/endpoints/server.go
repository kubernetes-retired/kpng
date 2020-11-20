package endpoints

import (
	"strconv"

	"github.com/cespare/xxhash"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/diffstore"
	"github.com/mcluseau/kube-proxy2/pkg/endpoints"
)

type Correlator interface {
	NextKVs(lastKnownRev uint64) (results []endpoints.KV, rev uint64)
}

type Server struct {
	Correlator Correlator
}

func (s *Server) Watch(res localnetv1.Endpoints_WatchServer) (srverr error) {
	// a few utilities to make the code easier to read
	send := func(item *localnetv1.OpItem) {
		if srverr != nil {
			return
		}
		err := res.Send(item)
		if err != nil {
			srverr = grpc.Errorf(codes.Aborted, "send error: %v", err)
		}
	}

	syncItem := &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{}}

	pb := proto.NewBuffer(nil)

	sendSet := func(set localnetv1.Set, path string, m proto.Message) {
		pb.Reset()
		if err := pb.Marshal(m); err != nil {
			panic("protobuf Marshal failed: " + err.Error())
		}

		send(&localnetv1.OpItem{
			Op: &localnetv1.OpItem_Set{
				Set: &localnetv1.Value{
					Ref:   &localnetv1.Ref{Set: set, Path: path},
					Bytes: pb.Bytes(),
				},
			},
		})
	}
	sendDelete := func(set localnetv1.Set, path string) {
		send(&localnetv1.OpItem{
			Op: &localnetv1.OpItem_Delete{
				Delete: &localnetv1.Ref{Set: set, Path: path},
			},
		})
	}

	hashOf := func(m proto.Message) uint64 {
		pb.Reset()
		if err := pb.Marshal(m); err != nil {
			panic("protobuf Marshal failed: " + err.Error())
		}

		return xxhash.Sum64(pb.Bytes())
	}

	// let's diff
	svcs := diffstore.New()
	seps := diffstore.New()

	var rev uint64
	for {
		// wait for client request
		req, err := res.Recv()
		if err != nil {
			return grpc.Errorf(codes.Aborted, "recv error: %v", err)
		}

		var list []endpoints.KV
		list, rev = s.Correlator.NextKVs(rev)
		rev++

		// initialize diffs
		svcs.Reset()
		seps.Reset()

		// set all new values
		for _, kv := range list {
			key := []byte(kv.Namespace + "/" + kv.Name)
			svcs.Set(key, hashOf(kv.Endpoints.Service), kv.Endpoints.Service)

			// filter endpoints for this node
			endpoints := kv.Endpoints.Endpoints
			// TODO again

			// filter endpoints by conditions
			if conds := req.RequiredEndpointConditions; conds != nil {
				eps := make([]*localnetv1.Endpoint, 0, len(endpoints))
				for _, ep := range endpoints {
					if !conds.Accept(ep.Conditions) {
						continue
					}
					eps = append(eps, ep)
				}

				endpoints = eps
			}

			for _, ep := range endpoints {
				// key is service key + endpoint hash (64 bits, in hex)
				key := append(make([]byte, 0, len(key)+1+64/8*2), key...)
				key = append(key, '/')

				h := hashOf(ep)
				key = strconv.AppendUint(key, h, 16)

				seps.Set(key, h, ep)
			}

		}

		for _, kv := range svcs.Updated() {
			sendSet(localnetv1.Set_ServicesSet, string(kv.Key), kv.Value.(*localnetv1.Service))
		}
		for _, kv := range seps.Updated() {
			sendSet(localnetv1.Set_EndpointsSet, string(kv.Key), kv.Value.(*localnetv1.Endpoint))
		}
		for _, kv := range seps.Deleted() {
			sendDelete(localnetv1.Set_EndpointsSet, string(kv.Key))
		}
		for _, kv := range svcs.Deleted() {
			sendDelete(localnetv1.Set_ServicesSet, string(kv.Key))
		}

		// change set sent
		send(syncItem)

		if srverr != nil {
			return
		}
	}
}
