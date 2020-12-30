package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/protobuf/proto"

	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
	"m.cluseau.fr/kube-proxy2/pkg/client"
)

func main() {
	epc, once, nodeName, _ := client.Default()

	for {
		if canceled := run(epc, once, nodeName); canceled {
			break
		}

		if once {
			break
		}
	}
}

var conn *grpc.ClientConn

func run(epc *client.EndpointsClient, once bool, nodeName string) (canceled bool) {
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
			return true
		} else if err != nil {
			log.Print("failed to connect: ", err)
			time.Sleep(time.Second)
			return
		}

		conn = c
	}

	ctx := epc.Context()

	cli := localnetv1.NewEndpointsClient(conn)

	w, err := cli.Watch(ctx)
	if isCanceled(err) {
		return true
	} else if err != nil {
		log.Print("failed to start the watch: ", err)
		time.Sleep(time.Second)
		return
	}

	for {
		err = watchReq(w, nodeName)
		if isCanceled(err) {
			return true
		} else if err != nil {
			log.Print("watch request failed: ", err)
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

func watchReq(w localnetv1.Endpoints_WatchClient, nodeName string) error {
	fmt.Println("< req", nodeName, "at", time.Now())
	if err := w.Send(&localnetv1.WatchReq{NodeName: nodeName}); err != nil {
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
			}

			if v != nil {
				err = proto.Unmarshal(set.Bytes, v)
				if err != nil {
					log.Print("failed to parse value: ", err)
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
