# Windows Userspace backend

Windows userspace mode standalone out-of-tree backend. Uses `netsh` tools.
The communication is made via gRPC to kpng core.

## Flags

The following flags are available in the binary. 

* "bind-address", default: 0.0.0.0" - bind address
* "port-range", default: "36000-37000" - port address range
* "sync-period-duration", default: 15 seconds -  "sync period duration"
* "udp-idle-timeout", default: 10 seconds - "UDP idle timeout"

## Compilation

Compile with `go 1.18` and use the binary as a standalone service.

```
GOOS=windows go build -o winuserspace.exe ./...
```

NOTE: Must run with hostProcess if used direclty in the cluster.
