package userspacelin

import (
	"sigs.k8s.io/kpng/client/backendcmd"
)

func init() {
	backendcmd.Register("to-userspacelin", func() backendcmd.Cmd { return &Backend{} })
}
