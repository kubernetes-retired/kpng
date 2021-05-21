package api2local

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"k8s.io/klog"

	"sigs.k8s.io/kpng/localsink"
	"sigs.k8s.io/kpng/pkg/api/localnetv1"
	"sigs.k8s.io/kpng/pkg/apiwatch"
	"sigs.k8s.io/kpng/pkg/tlsflags"
)

type Job struct {
	apiwatch.Watch
	Sink localsink.Sink
}

func New(sink localsink.Sink) *Job {
	return &Job{
		Watch: apiwatch.Watch{
			TLSFlags: &tlsflags.Flags{},
		},
		Sink: sink,
	}
}

func (j *Job) Run(ctx context.Context) {
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
	local := localnetv1.NewEndpointsClient(conn)

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

func (j *Job) runLoop(watch localnetv1.Endpoints_WatchClient) (err error) {
	ctx := watch.Context()

	if err = ctx.Err(); err != nil {
		return
	}

	nodeName, err := j.Sink.WaitRequest()

	err = watch.Send(&localnetv1.WatchReq{
		NodeName: nodeName,
	})
	if err != nil {
		return
	}

	for {
		var op *localnetv1.OpItem
		op, err = watch.Recv()

		if err != nil {
			return
		}

		switch op.Op.(type) {
		case *localnetv1.OpItem_Reset_:
			j.Sink.Reset()

		default:
			err = j.Sink.Send(op)
			if err != nil {
				return
			}
		}

		if _, isSync := op.Op.(*localnetv1.OpItem_Sync); isSync {
			return
		}
	}
}
