package ebpf

import (
	"github.com/spf13/pflag"

	"k8s.io/klog"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
	"sigs.k8s.io/kpng/client/localsink/fullstate/fullstatepipe"
)

var ebc ebpfController

type backend struct {
	cfg localsink.Config
}

func init() {
	backendcmd.Register("to-ebpf", func() backendcmd.Cmd { return &backend{} })
}

func (s *backend) BindFlags(flags *pflag.FlagSet) {
}

func (s *backend) Reset() { /* noop */ }

// WaitRequest see localsink.Sink#WaitRequest
// func (s *backend) WaitRequest() (nodeName string, err error) {
// 	name, _ := os.Hostname()
// 	return name, nil
// }

func (s *backend) Setup() {
	ebc = ebpfSetup()
	klog.Info("Loading ebpf maps and program %+v", ebc)
}

func (b *backend) Sync() { /* no-op */ }

func (b *backend) Sink() localsink.Sink {
	sink := fullstate.New(&b.cfg)

	sink.Callback = fullstatepipe.New(fullstatepipe.ParallelSendSequenceClose,
		ebc.Callback,
	).Callback

	sink.SetupFcn = b.Setup

	return sink
}
