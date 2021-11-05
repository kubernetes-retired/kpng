package iptables

import (
//        "github.com/spf13/pflag"

        "sigs.k8s.io/kpng/client/backendcmd"
  //      "sigs.k8s.io/kpng/client/localsink"
//        "sigs.k8s.io/kpng/client/localsink/filterreset"
 //       "sigs.k8s.io/kpng/client/localsink/decoder"
)


func init() {
	backendcmd.Register("to-iptables", func() backendcmd.Cmd { return &Backend{} })
}

