package iptables

import (
        "sigs.k8s.io/kpng/client/backendcmd"
)


func init() {
	backendcmd.Register("to-iptables", func() backendcmd.Cmd { return &Backend{} })
}

