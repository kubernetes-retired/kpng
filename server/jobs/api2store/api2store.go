package api2store

import (
	"context"
	localnetv12 "sigs.k8s.io/kpng/server/pkg/api/localnetv1"
	apiwatch2 "sigs.k8s.io/kpng/server/pkg/apiwatch"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	"k8s.io/klog"
)

type Job struct {
	apiwatch2.Watch
	Store *proxystore2.Store
}

func (j *Job) Run(ctx context.Context) {
	defer j.Store.Close()

	for {
		err := j.run(ctx)

		if err == context.Canceled || grpc.Code(err) == codes.Canceled {
			klog.Info("context canceled, closing global watch")
			return
		}

		klog.Error("global watch error: ", err)
		time.Sleep(5 * time.Second) // TODO parameter?
	}
}

func (j *Job) run(ctx context.Context) (err error) {
	// connect to API
	conn, err := j.Dial()
	if err != nil {
		return
	}
	defer conn.Close()

	// watch global state
	global := localnetv12.NewGlobalClient(conn)

	watch, err := global.Watch(ctx)
	if err != nil {
		return
	}

	for {
		if ctx.Err() != nil {
			err = ctx.Err()
			return
		}

		watch.Send(&localnetv12.GlobalWatchReq{})

		todo := make([]func(tx *proxystore2.Tx), 0)

	recvLoop:
		for {
			var op *localnetv12.OpItem
			op, err = watch.Recv()

			if err != nil {
				return
			}

			var storeOp func(tx *proxystore2.Tx)

			switch v := op.Op.(type) {
			case *localnetv12.OpItem_Reset_:
				storeOp = func(tx *proxystore2.Tx) {
					tx.Reset()
				}

			case *localnetv12.OpItem_Set:
				var value proxystore2.Hashed

				switch v.Set.Ref.Set {
				case localnetv12.Set_GlobalNodeInfos:
					info := &localnetv12.NodeInfo{}
					err = proto.Unmarshal(v.Set.Bytes, info)
					value = info

				case localnetv12.Set_GlobalServiceInfos:
					info := &localnetv12.ServiceInfo{}
					err = proto.Unmarshal(v.Set.Bytes, info)
					value = info

				case localnetv12.Set_GlobalEndpointInfos:
					info := &localnetv12.EndpointInfo{}
					err = proto.Unmarshal(v.Set.Bytes, info)
					value = info
				}

				storeOp = func(tx *proxystore2.Tx) {
					tx.SetRaw(v.Set.Ref.Set, v.Set.Ref.Path, value)
				}

			case *localnetv12.OpItem_Delete:
				storeOp = func(tx *proxystore2.Tx) {
					tx.DelRaw(v.Delete.Set, v.Delete.Path)
				}

			case *localnetv12.OpItem_Sync:
				// break on sync
				break recvLoop
			}

			if err != nil {
				return
			}

			if storeOp != nil {
				todo = append(todo, storeOp)
			}
		}

		if len(todo) != 0 {
			j.Store.Update(func(tx *proxystore2.Tx) {
				for _, op := range todo {
					op(tx)
				}

				tx.SetSync(proxystore2.Services)
				tx.SetSync(proxystore2.Endpoints)
				tx.SetSync(proxystore2.Nodes)
			})
		}
	}
}
