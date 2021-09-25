package api2local

import (
	"context"
	localnetv12 "sigs.k8s.io/kpng/server/pkg/api/localnetv1"
	apiwatch2 "sigs.k8s.io/kpng/server/pkg/apiwatch"
	tlsflags2 "sigs.k8s.io/kpng/server/pkg/tlsflags"
	"time"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"k8s.io/klog"

	"sigs.k8s.io/kpng/server/localsink"
)

// Config helps building sink with the standard flags (sinks are not required to have a stable node-name, but most will have).
//
// Simplest usage:
//
//     type MySink struct {
//         api2local.Config
//     }
//
type Config struct {
	NodeName string
}

func (c *Config) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&c.NodeName, "node-name", "", "Node name override")
}

type Job struct {
	apiwatch2.Watch
	Sink localsink.Sink
}

func New(sink localsink.Sink) *Job {
	return &Job{
		Watch: apiwatch2.Watch{
			TLSFlags: &tlsflags2.Flags{},
		},
		Sink: sink,
	}
}

func (j *Job) Run(ctx context.Context) {
	j.Sink.Setup()

	for {
		err := j.run(ctx)

		if err == context.Canceled || grpc.Code(err) == codes.Canceled {
			klog.Info("context canceled, closing global watch")
			return
		}

		klog.Error("local watch error: ", err)
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

	// watch local state
	local := localnetv12.NewEndpointsClient(conn)

	watch, err := local.Watch(ctx)
	if err != nil {
		return
	}

	for {
		err = j.runLoop(watch)
		if err != nil {
			return
		}
	}
}

func (j *Job) runLoop(watch localnetv12.Endpoints_WatchClient) (err error) {
	ctx := watch.Context()

	if err = ctx.Err(); err != nil {
		return
	}

	nodeName, err := j.Sink.WaitRequest()

	err = watch.Send(&localnetv12.WatchReq{
		NodeName: nodeName,
	})
	if err != nil {
		return
	}

	for {
		var op *localnetv12.OpItem
		op, err = watch.Recv()

		if err != nil {
			return
		}

		switch op.Op.(type) {
		case *localnetv12.OpItem_Reset_:
			j.Sink.Reset()

		default:
			err = j.Sink.Send(op)
			if err != nil {
				return
			}
		}

		if _, isSync := op.Op.(*localnetv12.OpItem_Sync); isSync {
			return
		}
	}
}
