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

package client

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"k8s.io/klog/v2"

	// allow multi gRPC URLs
	//_ "github.com/Jille/grpc-multi-resolver"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
	"sigs.k8s.io/kpng/client/pkg/tlsflags"
)

type ServiceEndpoints = fullstate.ServiceEndpoints

type FlagSet = tlsflags.FlagSet

// New returns a new EndpointsClient with values bound to the given flag-set for command-line tools.
// Other needs can use `&EndpointsClient{...}` directly.
func New(flags FlagSet) (epc *EndpointsClient) {
	epc = &EndpointsClient{
		TLS: &tlsflags.Flags{},
	}
	epc.ctx, epc.cancel = context.WithCancel(context.Background())
	epc.DefaultFlags(flags)
	return
}

// EndpointsClient is a simple client to kube-proxy's Endpoints API.
type EndpointsClient struct {
	// Target is the gRPC dial target
	Target string

	TLS *tlsflags.Flags

	// ErrorDelay is the delay before retrying after an error.
	ErrorDelay time.Duration

	// GRPCBuffer is the max size of a gRPC message
	MaxMsgSize int

	Sink localsink.Sink

	conn     *grpc.ClientConn
	watch    localnetv1.Endpoints_WatchClient
	watchReq *localnetv1.WatchReq

	ctx    context.Context
	cancel func()
}

// DefaultFlags registers this client's values to the standard flags.
func (epc *EndpointsClient) DefaultFlags(flags FlagSet) {
	flags.StringVar(&epc.Target, "target", "127.0.0.1:12090", "API to reach (can use multi:///1.0.0.1:1234,1.0.0.2:1234)")

	flags.DurationVar(&epc.ErrorDelay, "error-delay", 1*time.Second, "duration to wait before retrying after errors")

	flags.IntVar(&epc.MaxMsgSize, "max-msg-size", 4<<20, "max gRPC message size")

	epc.TLS.Bind(flags, "")
}

// Next sends the next diff to the sink, waiting for a new revision as needed.
// It's designed to never fail, unless canceled.
func (epc *EndpointsClient) Next() (canceled bool) {
	if epc.watch == nil {
		epc.dial()
	}

retry:
	if epc.ctx.Err() != nil {
		canceled = true
		return
	}

	// say we're ready
	nodeName, err := epc.Sink.WaitRequest()
	if err != nil { // errors are considered as cancel
		canceled = true
		return
	}

	err = epc.watch.Send(&localnetv1.WatchReq{
		NodeName: nodeName,
	})
	if err != nil {
		epc.postError()
		goto retry
	}

	for {
		op, err := epc.watch.Recv()

		if err != nil {
			// klog.Error("watch recv failed: ", err)
			epc.postError()
			goto retry
		}

		// pass the op to the sync
		epc.Sink.Send(op)

		// break on sync
		switch v := op.Op; v.(type) {
		case *localnetv1.OpItem_Sync:
			return
		}
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

		sig = <-c
		klog.Info("got signal ", sig, " again, forcing exit")
		os.Exit(1)
	}()
}

func (epc *EndpointsClient) Context() context.Context {
	return epc.ctx
}

func (epc *EndpointsClient) DialContext(ctx context.Context) (conn *grpc.ClientConn, err error) {
	klog.Info("connecting to ", epc.Target)

	opts := append(
		make([]grpc.DialOption, 0),
		grpc.WithMaxMsgSize(epc.MaxMsgSize),
	)

	tlsCfg := epc.TLS.Config()
	if tlsCfg == nil {
		opts = append(opts, grpc.WithInsecure())
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}

	return grpc.DialContext(epc.ctx, epc.Target, opts...)
}

func (epc *EndpointsClient) Dial() (conn *grpc.ClientConn, err error) {
	if ctxErr := epc.ctx.Err(); ctxErr == context.Canceled {
		err = ctxErr
		return
	}

	return epc.DialContext(epc.ctx)
}

func (epc *EndpointsClient) dial() (canceled bool) {
retry:
	conn, err := epc.Dial()

	if err == context.Canceled {
		return true
	} else if err != nil {
		//klog.Info("failed to connect: ", err)
		epc.errorSleep()
		goto retry
	}

	epc.conn = conn
	epc.watch, err = localnetv1.NewEndpointsClient(epc.conn).Watch(epc.ctx)

	if err != nil {
		conn.Close()

		//klog.Info("failed to start watch: ", err)
		epc.errorSleep()
		goto retry
	}

	epc.Sink.Reset()

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
