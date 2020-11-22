package client

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"k8s.io/klog"

	"github.com/google/btree"
	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

type ServiceEndpoints struct {
	Service   *localnetv1.Service
	Endpoints []*localnetv1.Endpoint
}

// FlagSet matches flag.FlagSet and pflag.FlagSet
type FlagSet interface {
	DurationVar(varPtr *time.Duration, name string, value time.Duration, doc string)
	IntVar(varPtr *int, name string, value int, doc string)
	StringVar(varPtr *string, name, value, doc string)
	Uint64Var(varPtr *uint64, name string, value uint64, doc string)
}

// New returns a new EndpointsClient with values bound to the given flag-set for command-line tools.
// Other needs can use `&EndpointsClient{...}` directly.
func New(flags FlagSet) (epc *EndpointsClient) {
	epc = &EndpointsClient{}
	epc.ctx, epc.cancel = context.WithCancel(context.Background())
	epc.DefaultFlags(flags)
	return
}

// EndpointsClient is a simple client to kube-proxy's Endpoints API.
type EndpointsClient struct {
	// Target is the gRPC dial target
	Target string

	// ErrorDelay is the delay before retrying after an error.
	ErrorDelay time.Duration

	// GRPCBuffer is the max size of a gRPC message
	MaxMsgSize int

	conn     *grpc.ClientConn
	watch    localnetv1.Endpoints_WatchClient
	watchReq *localnetv1.WatchReq

	data *btree.BTree

	ctx    context.Context
	cancel func()
}

// DefaultFlags registers this client's values to the standard flags.
func (epc *EndpointsClient) DefaultFlags(flags FlagSet) {
	flags.StringVar(&epc.Target, "target", "127.0.0.1:12090", "local API to reach")

	flags.DurationVar(&epc.ErrorDelay, "error-delay", 1*time.Second, "duration to wait before retrying after errors")

	flags.IntVar(&epc.MaxMsgSize, "max-msg-size", 4<<20, "max gRPC message size")
}

// Next returns the next set of ServiceEndpoints, waiting for a new revision as needed.
// It's designed to never fail and will always return latest items, unless canceled.
func (epc *EndpointsClient) Next(req *localnetv1.WatchReq) (items []*ServiceEndpoints, canceled bool) {
	ch, canceled := epc.NextCh(req)

	if canceled {
		return
	}

	items = make([]*ServiceEndpoints, 0, epc.data.Len())

	for seps := range ch {
		items = append(items, seps)
	}

	return
}

// NextCh returns the next set of ServiceEndpoints as a channel, waiting for a new revision as needed.
// It's designed to never fail and will always return an valid channel, unless canceled.
func (epc *EndpointsClient) NextCh(req *localnetv1.WatchReq) (results chan *ServiceEndpoints, canceled bool) {
	results = make(chan *ServiceEndpoints, 100)

	if epc.watch == nil {
		epc.dial()
	}

retry:
	if epc.ctx.Err() != nil {
		canceled = true
		close(results)
		return
	}

	// say we're ready
	err := epc.watch.Send(req)
	if err != nil {
		epc.postError()
		goto retry
	}

	// apply diff
apply:
	for {
		op, err := epc.watch.Recv()

		if err != nil {
			klog.Error("watch recv failed: ", err)
			epc.postError()
			goto retry
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

			default:
				continue apply // ignore unknown set
			}

			err = proto.Unmarshal(set.Bytes, v)
			if err != nil {
				klog.Error("failed to parse value: ", err)
				continue apply
			}

			epc.data.ReplaceOrInsert(kv{set.Ref.Path, v})

		case *localnetv1.OpItem_Delete:
			epc.data.Delete(kv{Path: op.GetDelete().Path})

		case *localnetv1.OpItem_Sync:
			break apply // done
		}
	}

	go func() {
		defer close(results)

		var seps *ServiceEndpoints

		epc.data.Ascend(func(i btree.Item) bool {
			switch v := i.(kv).Value.(type) {
			case *localnetv1.Service:
				if seps != nil {
					results <- seps
				}

				seps = &ServiceEndpoints{Service: v}
			case *localnetv1.Endpoint:
				seps.Endpoints = append(seps.Endpoints, v)
			}

			return true
		})

		if seps != nil {
			results <- seps
		}
	}()

	return
}

// Cancel will cancel this client, quickly closing any call to Next.
func (epc *EndpointsClient) Cancel() {
	epc.cancel()
}

// CancelOnSignals make the default termination signals to cancel this client.
func (epc *EndpointsClient) CancelOnSignals() {
	epc.CancelOn(os.Interrupt, os.Kill, syscall.SIGTERM)
}

// CancelOn make the given signals to cancel this client.
func (epc *EndpointsClient) CancelOn(signals ...os.Signal) {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, signals...)
		sig := <-c

		klog.Info("got signal ", sig, ", stopping")
		epc.Cancel()

		sig = <-c
		klog.Info("got signal ", sig, " again, forcing exit")
		os.Exit(1)
	}()
}

func (epc *EndpointsClient) dial() (canceled bool) {
retry:
	if epc.ctx.Err() == context.Canceled {
		return true
	}

	klog.Info("connecting to ", epc.Target)

	conn, err := grpc.DialContext(epc.ctx, epc.Target, grpc.WithInsecure(), grpc.WithMaxMsgSize(epc.MaxMsgSize))

	if err != nil {
		klog.Info("failed to connect: ", err)
		epc.errorSleep()
		goto retry
	}

	epc.conn = conn
	epc.watch, err = localnetv1.NewEndpointsClient(epc.conn).Watch(epc.ctx)

	if err != nil {
		conn.Close()

		klog.Info("failed to start watch: ", err)
		epc.errorSleep()
		goto retry
	}

	epc.data = btree.New(2)

	//klog.V(1).Info("connected")
	return false
}

func (epc *EndpointsClient) errorSleep() {
	time.Sleep(epc.ErrorDelay)
}

func (epc *EndpointsClient) postError() {
	if epc.watch != nil {
		epc.watch.CloseSend()
		epc.watch = nil
	}

	if epc.conn != nil {
		epc.conn.Close()
		epc.conn = nil
	}

	if err := epc.ctx.Err(); err != nil {
		return
	}

	epc.errorSleep()
	epc.dial()
}
