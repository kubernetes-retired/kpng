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
	"k8s.io/klog"

	"sigs.k8s.io/kpng/api/localnetv1"
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
	epc = client.New(flags)

	cmd.Execute()
}

var (
	epc  *client.EndpointsClient
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
		c, err := epc.Dial()
		if isCanceled(err) {
			return
		} else if err != nil {
			klog.Info("failed to connect: ", err)
			time.Sleep(time.Second)
			return
		}

		conn = c
	}

	ctx := epc.Context()

	cli := localnetv1.NewGlobalClient(conn)

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

func watchReq(w localnetv1.Global_WatchClient) error {
	fmt.Println("< req (global) at", time.Now())
	if err := w.Send(&localnetv1.GlobalWatchReq{}); err != nil {
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
		case *localnetv1.OpItem_Set:
			set := op.GetSet()

			var v proto.Message
			switch set.Ref.Set {
			case localnetv1.Set_ServicesSet:
				v = &localnetv1.Service{}
			case localnetv1.Set_EndpointsSet:
				v = &localnetv1.Endpoint{}

			case localnetv1.Set_GlobalEndpointInfos:
				v = &localnetv1.EndpointInfo{}
			case localnetv1.Set_GlobalNodeInfos:
				v = &localnetv1.NodeInfo{}
			case localnetv1.Set_GlobalServiceInfos:
				v = &localnetv1.ServiceInfo{}

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

		case *localnetv1.OpItem_Delete:
			refStr := op.GetDelete().String()
			prev := prevs[refStr]

			fmt.Println("-", refStr, "->", prev)
			delete(prevs, refStr)

		case *localnetv1.OpItem_Sync:
			fmt.Println("> sync after", time.Since(start))
			break loop // done
		}
	}

	return nil
}

func isCanceled(err error) bool {
	return err == context.Canceled || grpc.Code(err) == codes.Canceled
}
