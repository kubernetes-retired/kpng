package backendcmd

import (
	"github.com/spf13/pflag"

	"sigs.k8s.io/kpng/client/localsink"
)

type Cmd interface {
	BindFlags(*pflag.FlagSet)
	Sink() localsink.Sink
}

var registry []UseCmd

type UseCmd struct {
	Use string
	New func() Cmd
}

func Register(use string, new func() Cmd) {
	registry = append(registry, UseCmd{Use: use, New: new})
}

func Registered() []UseCmd {
	return registry
}
