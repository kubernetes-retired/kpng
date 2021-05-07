package api2store

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"

	"k8s.io/klog"

	"sigs.k8s.io/kpng/pkg/api/localnetv1"
	"sigs.k8s.io/kpng/pkg/proxystore"
	"sigs.k8s.io/kpng/pkg/tlsflags"
)

type Job struct {
	Server   string
	TLSFlags *tlsflags.Flags
	Store    *proxystore.Store
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
	opts := []grpc.DialOption{}

	if j.TLSFlags != nil && j.TLSFlags.KeyFile != "" {
		tlsConfig := j.TLSFlags.Config()
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.Dial(j.Server, opts...)
	if err != nil {
		return
	}

	defer conn.Close()

	global := localnetv1.NewGlobalClient(conn)

	watch, err := global.Watch(ctx)
	if err != nil {
		return
	}

	for {
		if ctx.Err() != nil {
			err = ctx.Err()
			return
		}

		watch.Send(&localnetv1.GlobalWatchReq{})

		todo := make([]func(tx *proxystore.Tx), 0)

	recvLoop:
		for {
			var op *localnetv1.OpItem
			op, err = watch.Recv()

			if err != nil {
				return
			}

			var storeOp func(tx *proxystore.Tx)

			switch v := op.Op.(type) {
			case *localnetv1.OpItem_Reset_:
				storeOp = func(tx *proxystore.Tx) {
					tx.Reset()
				}

			case *localnetv1.OpItem_Set:
				var value proxystore.Hashed

				switch v.Set.Ref.Set {
				case localnetv1.Set_GlobalNodeInfos:
					info := &localnetv1.NodeInfo{}
					err = proto.Unmarshal(v.Set.Bytes, info)
					value = info

				case localnetv1.Set_GlobalServiceInfos:
					info := &localnetv1.ServiceInfo{}
					err = proto.Unmarshal(v.Set.Bytes, info)
					value = info

				case localnetv1.Set_GlobalEndpointInfos:
					info := &localnetv1.EndpointInfo{}
					err = proto.Unmarshal(v.Set.Bytes, info)
					value = info
				}

				storeOp = func(tx *proxystore.Tx) {
					tx.SetRaw(v.Set.Ref.Set, v.Set.Ref.Path, value)
				}

			case *localnetv1.OpItem_Delete:
				storeOp = func(tx *proxystore.Tx) {
					tx.DelRaw(v.Delete.Set, v.Delete.Path)
				}

			case *localnetv1.OpItem_Sync:
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
			j.Store.Update(func(tx *proxystore.Tx) {
				for _, op := range todo {
					op(tx)
				}

				tx.SetSync(proxystore.Services)
				tx.SetSync(proxystore.Endpoints)
				tx.SetSync(proxystore.Nodes)
			})
		}
	}
}
