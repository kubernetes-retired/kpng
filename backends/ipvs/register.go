package ipvs

import (
	"github.com/spf13/pflag"

	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

type backend struct{
    cfg localsink.Config
}

func init() {
	backendcmd.Register("to-ipvs-exec", func() backendcmd.Cmd { return &backend{} })
}

func (b *backend) BindFlags(flags *pflag.FlagSet) {
    b.cfg.BindFlags(flags)
	BindFlags(flags)
}

func (b *backend) Sink() localsink.Sink {
	sink := fullstate.New(&b.cfg)

	PreRun()
	sink.Callback = Callback

	return sink
}
