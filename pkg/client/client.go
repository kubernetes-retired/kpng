package client

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"k8s.io/klog"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

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

	// InstanceID and Rev are the latest known state (used to resume a watch)
	InstanceID uint64
	Rev        uint64

	// ErrorDelay is the delay before retrying after an error.
	ErrorDelay time.Duration

	// GRPCBuffer is the max size of a gRPC message
	MaxMsgSize int

	conn       *grpc.ClientConn
	client     localnetv1.EndpointsClient
	nextFilter *localnetv1.NextFilter

	prevLen int

	ctx    context.Context
	cancel func()
}

// DefaultFlags registers this client's values to the standard flags.
func (epc *EndpointsClient) DefaultFlags(flags FlagSet) {
	flags.StringVar(&epc.Target, "target", "127.0.0.1:12090", "local API to reach")

	flags.Uint64Var(&epc.InstanceID, "instance-id", 0, "Instance ID (to resume a watch)")
	flags.Uint64Var(&epc.Rev, "rev", 0, "Rev (to resume a watch)")

	flags.DurationVar(&epc.ErrorDelay, "error-delay", 1*time.Second, "duration to wait before retrying after errors")

	flags.IntVar(&epc.MaxMsgSize, "max-msg-size", 4<<20, "max gRPC message size")
}

// Next returns the next set of ServiceEndpoints, waiting for a new revision as needed.
// It's designed to never fail and will always return latest items, unless canceled.
func (epc *EndpointsClient) Next() (items []*localnetv1.ServiceEndpoints, canceled bool) {
	// prepare items assuming a 10% increase of the number of items
	expectedCap := epc.prevLen * 110 / 100
	if expectedCap == 0 {
		expectedCap = 10
	}

	items = make([]*localnetv1.ServiceEndpoints, 0, expectedCap)

retry:
	iter := epc.NextIterator()

	for seps := range iter.Ch {
		items = append(items, seps)
	}

	if iter.RecvErr != nil {
		klog.Warning("recv error: ", iter.RecvErr)
		items = items[:0]
		goto retry
	}

	canceled = iter.Canceled
	return
}

// NextIterator returns the next set of ServiceEndpoints as an iterator, waiting for a new revision as needed.
// It's designed to never fail and will always return an valid iterator (than may be canceled or end on error)
func (epc *EndpointsClient) NextIterator() (iter *Iterator) {
	results := make(chan *localnetv1.ServiceEndpoints, 100)

	iter = &Iterator{
		Ch: results,
	}

	if epc.conn == nil {
		if canceled := epc.dial(); canceled {
			iter.Canceled = true
			close(results)
			return
		}
	}

	if epc.nextFilter == nil {
		epc.nextFilter = &localnetv1.NextFilter{
			InstanceID: epc.InstanceID,
			Rev:        epc.Rev,
		}
	}

	ctx := epc.ctx

	count := 0

	for {
		next, err := epc.client.Next(ctx, epc.nextFilter)
		if err != nil {
			if epc.ctx.Err() == context.Canceled {
				iter.Canceled = true
				close(results)
				return
			}

			klog.Error("next failed: ", err)
			epc.postError()
			continue
		}

		nextItem, err := next.Recv()
		if err != nil {
			epc.postError()
			continue
		}

		nextFilter := nextItem.Next

		go func() {
			defer close(results)
			for {
				nextItem, err := next.Recv()

				if err == io.EOF {
					break
				} else if err != nil {
					iter.RecvErr = err
					return
				}

				results <- nextItem.Endpoints
				count++
			}
		}()

		epc.nextFilter = nextFilter
		epc.prevLen = count
		return
	}
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
	epc.client = localnetv1.NewEndpointsClient(epc.conn)

	//klog.V(1).Info("connected")
	return false
}

func (epc *EndpointsClient) errorSleep() {
	time.Sleep(epc.ErrorDelay)
}

func (epc *EndpointsClient) postError() {
	epc.errorSleep()

	if epc.conn != nil {
		epc.conn.Close()
		epc.conn = nil

		epc.dial()
	}
}
