# Store Commands

The KPNG `storecmds` package has the "glue" which connects
the KPNG server (which talkes to K8s), to the a proxy backend.

## How can I use the store programs in KPNG ?

The "store" programs in this package can be used to respond to events
coming in from the Kubernetes API Server that KPNG cares about.

- api2store: TODO add definition
- file2store: polling files, and writing them to KPNGs global state.
- kube2store: polling Kubernetes API, and writing them to KPNGs global state.
- local2sink: 

## Who should add a store ?

KPNG is designed for external consumers to build their
own backend logic for dealing with Kubernetes services.

Thus, it's not common to build a new store, since most of the
store's in KPNG are really offered for backwards compatibility
and as examples of how KPNG relates to the original, in-tree kube proxy.


## Example

Right now, the store commands are dynamically loaded from the backends that are in-tree
for KPNG.  For example, the iptables backend has an init function which registes the
`to-iptables` flag, so that iptables backend can automatically be started up by
KPNG.

```
package iptables

import (
        "sigs.k8s.io/kpng/client/backendcmd"
)

func init() {
	backendcmd.Register("to-iptables", func() backendcmd.Cmd { return &Backend{} })
}
```