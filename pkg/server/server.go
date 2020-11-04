package server

import (
	"net"
	"os"
	"strings"
	"syscall"

	"k8s.io/klog"
)

func MustListen(bindSpec string) net.Listener {
	parts := strings.SplitN(bindSpec, "://", 2)
	if len(parts) != 2 {
		klog.Error("invalid listen spec: expected protocol://address format but got ", bindSpec)
		os.Exit(1)
	}

	protocol, addr := parts[0], parts[1]

	// handle protocol specifics
	afterListen := func() {}
	switch protocol {
	case "unix":
		os.Remove(addr)
		prevMask := syscall.Umask(0007)
		afterListen = func() { syscall.Umask(prevMask) }
	}

	lis, err := net.Listen(protocol, addr)
	if err != nil {
		klog.Error("failed to listen on ", bindSpec, ": ", err)
		os.Exit(1)
	}

	afterListen()

	klog.Info("listening on ", bindSpec)

	return lis
}
