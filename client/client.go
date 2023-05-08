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

	// allow multi gRPC URLs
	//_ "github.com/Jille/grpc-multi-resolver"

	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
	"sigs.k8s.io/kpng/client/tlsflags"
)

type ServiceEndpoints = fullstate.ServiceEndpoints

type FlagSet = tlsflags.FlagSet

// New returns a new LocalClient with values bound to the given flag-set for command-line tools.
// Other needs can use `&LocalClient{...}` directly.
func New(flags FlagSet) (lc *LocalClient) {
	lc = &LocalClient{
		TLS: &tlsflags.Flags{},
	}
	lc.ctx, lc.cancel = context.WithCancel(context.Background())
	lc.DefaultFlags(flags)
	return
}

// LocalClient is a simple client to kube-proxy's Endpoints API.
type LocalClient struct {
	// Target is the gRPC dial target
	Target string

	TLS *tlsflags.Flags

	// ErrorDelay is the delay before retrying after an error.
	ErrorDelay time.Duration

	// GRPCBuffer is the max size of a gRPC message
	MaxMsgSize int

	Sink localsink.Sink

	conn     *grpc.ClientConn
	watch    localv1.Sets_WatchClient
	watchReq *localv1.WatchReq

	ctx    context.Context
	cancel func()
}

// DefaultFlags registers this client's values to the standard flags.
func (lc *LocalClient) DefaultFlags(flags FlagSet) {
	flags.StringVar(&lc.Target, "api", "127.0.0.1:12090", "API to reach (can use multi:///1.0.0.1:1234,1.0.0.2:1234)")
	flags.DurationVar(&lc.ErrorDelay, "error-delay", 1*time.Second, "duration to wait before retrying after errors")
	flags.IntVar(&lc.MaxMsgSize, "max-msg-size", 4<<20, "max gRPC message size")

	lc.TLS.Bind(flags, "")
}

// Next sends the next diff to the sink, waiting for a new revision as needed.
// It's designed to never fail, unless canceled.
func (lc *LocalClient) Next() (canceled bool) {
	if lc.watch == nil {
		lc.dial()
	}

retry:
	if lc.ctx.Err() != nil {
		canceled = true
		return
	}

	// say we're ready
	nodeName, err := lc.Sink.WaitRequest()
	if err != nil { // errors are considered as cancel
		canceled = true
		return
	}

	err = lc.watch.Send(&localv1.WatchReq{
		NodeName: nodeName,
	})
	if err != nil {
		lc.postError()
		goto retry
	}

	for {
		op, err := lc.watch.Recv()

		if err != nil {
			// klog.Error("watch recv failed: ", err)
			lc.postError()
			goto retry
		}

		// pass the op to the sync
		lc.Sink.Send(op)

		// break on sync
		switch v := op.Op; v.(type) {
		case *localv1.OpItem_Sync:
			return
		}
	}
}

// Cancel will cancel this client, quickly closing any call to Next.
func (lc *LocalClient) Cancel() {
	lc.cancel()
}

// CancelOnSignals make the default termination signals to cancel this client.
func (lc *LocalClient) CancelOnSignals() {
	lc.CancelOn(os.Interrupt, os.Kill, syscall.SIGTERM)
}

// CancelOn make the given signals to cancel this client.
func (lc *LocalClient) CancelOn(signals ...os.Signal) {
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, signals...)

		sig := <-c
		klog.Info("got signal ", sig, ", stopping")
		lc.Cancel()

		sig = <-c
		klog.Info("got signal ", sig, " again, forcing exit")
		os.Exit(1)
	}()
}

func (lc *LocalClient) Context() context.Context {
	return lc.ctx
}

func (lc *LocalClient) DialContext(ctx context.Context) (conn *grpc.ClientConn, err error) {
	klog.Info("connecting to ", lc.Target)

	opts := append(
		make([]grpc.DialOption, 0),
		grpc.WithMaxMsgSize(lc.MaxMsgSize),
	)

	tlsCfg := lc.TLS.Config()
	if tlsCfg == nil {
		opts = append(opts, grpc.WithInsecure())
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}

	return grpc.DialContext(lc.ctx, lc.Target, opts...)
}

func (lc *LocalClient) Dial() (conn *grpc.ClientConn, err error) {
	if ctxErr := lc.ctx.Err(); ctxErr == context.Canceled {
		err = ctxErr
		return
	}

	return lc.DialContext(lc.ctx)
}

func (lc *LocalClient) dial() (canceled bool) {
retry:
	conn, err := lc.Dial()

	if err == context.Canceled {
		return true
	} else if err != nil {
		//klog.Info("failed to connect: ", err)
		lc.errorSleep()
		goto retry
	}

	lc.conn = conn
	lc.watch, err = localv1.NewSetsClient(lc.conn).Watch(lc.ctx)

	if err != nil {
		conn.Close()

		//klog.Info("failed to start watch: ", err)
		lc.errorSleep()
		goto retry
	}

	lc.Sink.Reset()

	//klog.V(1).Info("connected")
	return false
}

func (lc *LocalClient) errorSleep() {
	time.Sleep(lc.ErrorDelay)
}

func (lc *LocalClient) postError() {
	if lc.watch != nil {
		lc.watch.CloseSend()
		lc.watch = nil
	}

	if lc.conn != nil {
		lc.conn.Close()
		lc.conn = nil
	}

	if err := lc.ctx.Err(); err != nil {
		return
	}

	lc.errorSleep()
	lc.dial()
}
