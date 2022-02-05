package nft

import (
	"github.com/spf13/pflag"

	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
	"sigs.k8s.io/kpng/client/localsink/fullstate/fullstatepipe"
	"sigs.k8s.io/kpng/client/plugins/conntrack"
)

type backend struct {
	cfg localsink.Config
}

func init() {
	backendcmd.Register("to-nft", func() backendcmd.Cmd { return &backend{} })
}

func (b *backend) BindFlags(flags *pflag.FlagSet) {
	b.cfg.BindFlags(flags)
	BindFlags(flags)
}

func (b *backend) Sink() localsink.Sink {
	sink := fullstate.New(&b.cfg)

	PreRun()

	ct := conntrack.New()
	sink.Callback = fullstatepipe.New(fullstatepipe.ParallelSendSequenceClose,
		Callback,
		ct.Callback,
	).Callback

	return sink
}
