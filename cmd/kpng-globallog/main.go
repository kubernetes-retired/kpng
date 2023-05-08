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

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/protobuf/proto"

	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/api/globalv1"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client"
)

func main() {
	cmd := &cobra.Command{
		Run: func(_ *cobra.Command, _ []string) {
			run()
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&once, "once", false, "run only one loop")
	lc = client.New(flags)

	cmd.Execute()
}

var (
	lc   *client.LocalClient
	conn *grpc.ClientConn
	once bool
)

func run() {
	if conn != nil {
		switch conn.GetState() {
		case connectivity.Shutdown:
			conn.Close()
			conn = nil
		}
	}

	if conn == nil {
		c, err := lc.Dial()
		if isCanceled(err) {
			return
		} else if err != nil {
			klog.Info("failed to connect: ", err)
			time.Sleep(time.Second)
			return
		}

		conn = c
	}

	ctx := lc.Context()

	cli := globalv1.NewSetsClient(conn)

	w, err := cli.Watch(ctx)
	if isCanceled(err) {
		return
	} else if err != nil {
		klog.Info("failed to start the watch: ", err)
		time.Sleep(time.Second)
		return
	}

	for {
		err = watchReq(w)
		if isCanceled(err) {
			return
		} else if err != nil {
			klog.Info("watch request failed: ", err)
			time.Sleep(time.Second)
			return
		}

		if once {
			break
		}
	}

	return
}

var prevs = map[string]proto.Message{}

func watchReq(w globalv1.Sets_WatchClient) error {
	fmt.Println("< req (globalv1) at", time.Now())
	if err := w.Send(&globalv1.GlobalWatchReq{}); err != nil {
		return err
	}

	start := time.Time{}

loop:
	for {
		op, err := w.Recv()
		if err != nil {
			return err
		}

		if start.IsZero() {
			start = time.Now()
			fmt.Println("< recv at", start)
		}

		switch v := op.Op; v.(type) {
		case *localv1.OpItem_Set:
			set := op.GetSet()

			var v proto.Message
			switch set.Ref.Set {
			case localv1.Set_ServicesSet:
				v = &localv1.Service{}
			case localv1.Set_EndpointsSet:
				v = &localv1.Endpoint{}

			case localv1.Set_GlobalEndpointInfos:
				v = &globalv1.EndpointInfo{}
			case localv1.Set_GlobalNodeInfos:
				v = &globalv1.NodeInfo{}
			case localv1.Set_GlobalServiceInfos:
				v = &globalv1.ServiceInfo{}

			default:
				klog.Info("unknown set: ", set.Ref.Set)
				continue loop
			}

			if v != nil {
				err = proto.Unmarshal(set.Bytes, v)
				if err != nil {
					klog.Info("failed to parse value: ", err)
					v = nil
				}
			}

			refStr := set.Ref.String()
			if prev, ok := prevs[refStr]; ok {
				fmt.Println("-", refStr, "->", prev)
			}
			fmt.Println("+", refStr, "->", v)

			prevs[refStr] = v

		case *localv1.OpItem_Delete:
			refStr := op.GetDelete().String()
			prev := prevs[refStr]

			fmt.Println("-", refStr, "->", prev)
			delete(prevs, refStr)

		case *localv1.OpItem_Sync:
			fmt.Println("> sync after", time.Since(start))
			break loop // done
		}
	}

	return nil
}

func isCanceled(err error) bool {
	return err == context.Canceled || grpc.Code(err) == codes.Canceled
}
