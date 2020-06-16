package client

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"k8s.io/klog"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

// FlagSet matches flag.FlagSet and pflag.FlagSet
type FlagSet interface {
	StringVar(varPtr *string, name, value, doc string)
	Uint64Var(varPtr *uint64, name string, value uint64, doc string)
	DurationVar(varPtr *time.Duration, name string, value time.Duration, doc string)
}

// New returns a new EndpointsClient with values bound to the given flag-set for command-line tools.
// Other needs can use `&EndpointsClient{...}` directly.
func New(flags FlagSet) (epc *EndpointsClient) {
	epc = &EndpointsClient{}
	epc.DefaultFlags(flags)
	return
}

// EndpointsClient is a simple client to kube-proxy's Endpoints API.
type EndpointsClient struct {
	Target string

	InstanceID uint64
	Rev        uint64

	ErrorDelay time.Duration

	conn       *grpc.ClientConn
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
}

// Next returns the next set of ServiceEndpoints, waiting for a new revision as needed.
// It's designed to never fail and will only return nil if this client is canceled.
func (epc *EndpointsClient) Next() []*localnetv1.ServiceEndpoints {
	if epc.conn == nil {
		if canceled := epc.dial(); canceled {
			return nil
		}
	}

	ctx := epc.ctx

	if ctx == nil {
		ctx, epc.cancel = context.WithCancel(context.Background())
		epc.ctx = ctx
	}

	if epc.nextFilter == nil {
		epc.nextFilter = &localnetv1.NextFilter{
			InstanceID: epc.InstanceID,
			Rev:        epc.Rev,
		}
	}

	client := localnetv1.NewEndpointsClient(epc.conn)

	// prepare items assuming a mild increase of the number of items
	expectedCap := epc.prevLen * 110 / 100
	if expectedCap == 0 {
		expectedCap = 10
	}

	items := make([]*localnetv1.ServiceEndpoints, 0, expectedCap)

	for {
		next, err := client.Next(ctx, epc.nextFilter)
		if err != nil {
			if grpc.Code(err) == codes.Canceled || err == context.Canceled {
				return nil
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

		epc.nextFilter = nextItem.Next

		items = items[:0]
		for {
			nextItem, err := next.Recv()

			if err == io.EOF {
				break
			} else if err != nil {
				klog.Error("recv failed: ", err)
			}

			items = append(items, nextItem.Endpoints)
		}

		epc.prevLen = len(items)
		return items
	}
}

// Cancel will cancel this client, quickly closing any call to Next.
func (epc *EndpointsClient) Cancel() {
	cancel := epc.cancel
	if cancel != nil {
		epc.cancel = nil
		epc.ctx = nil
		cancel()
	}
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
	klog.Info("connecting to ", epc.Target)

retry:
	conn, err := grpc.Dial(epc.Target, grpc.WithInsecure())

	if err == context.Canceled {
		return true
	} else if err != nil {
		klog.Info("failed to connect: ", err)
		epc.errorSleep()
		goto retry
	}

	epc.conn = conn
	klog.V(1).Info("connected")
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
