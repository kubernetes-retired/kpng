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

package api2store

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/api/globalv1"
	"sigs.k8s.io/kpng/server/pkg/apiwatch"
	"sigs.k8s.io/kpng/server/proxystore"
)

type Job struct {
	apiwatch.Watch
	Store *proxystore.Store
}

func (j *Job) Run(ctx context.Context) {
	defer j.Store.Close()

	for {
		err := j.run(ctx)

		if err == context.Canceled || grpc.Code(err) == codes.Canceled {
			klog.Info("context canceled, closing watch")
			return
		}

		klog.Error("watch error: ", err)
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

	// watch globalv1 state
	global := globalv1.NewSetsClient(conn)

	watch, err := global.Watch(ctx)
	if err != nil {
		return
	}

	for {
		if ctx.Err() != nil {
			err = ctx.Err()
			return
		}

		watch.Send(&globalv1.GlobalWatchReq{})

		todo := make([]func(tx *proxystore.Tx), 0)

	recvLoop:
		for {
			var op *localv1.OpItem
			op, err = watch.Recv()

			if err != nil {
				return
			}

			var storeOp func(tx *proxystore.Tx)

			switch v := op.Op.(type) {
			case *localv1.OpItem_Reset_:
				storeOp = func(tx *proxystore.Tx) {
					tx.Reset()
				}

			case *localv1.OpItem_Set:
				var value proxystore.Hashed

				switch v.Set.Ref.Set {
				case localv1.Set_GlobalNodeInfos:
					info := &globalv1.NodeInfo{}
					err = proto.Unmarshal(v.Set.Bytes, info)
					value = info

				case localv1.Set_GlobalServiceInfos:
					info := &globalv1.ServiceInfo{}
					err = proto.Unmarshal(v.Set.Bytes, info)
					value = info

				case localv1.Set_GlobalEndpointInfos:
					info := &globalv1.EndpointInfo{}
					err = proto.Unmarshal(v.Set.Bytes, info)
					value = info
				}

				storeOp = func(tx *proxystore.Tx) {
					tx.SetRaw(v.Set.Ref.Set, v.Set.Ref.Path, value)
				}

			case *localv1.OpItem_Delete:
				storeOp = func(tx *proxystore.Tx) {
					tx.DelRaw(v.Delete.Set, v.Delete.Path)
				}

			case *localv1.OpItem_Sync:
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
