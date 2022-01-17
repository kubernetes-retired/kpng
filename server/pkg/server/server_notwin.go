//go:build !windows
// +build !windows

package server

import (
	"os"
	"syscall"
)

func osPrepareListen(protocol, addr string) func() {
	switch protocol {
	case "unix":
		os.Remove(addr)
		prevMask := syscall.Umask(0007)
		return func() { syscall.Umask(prevMask) }
	}

	return func() {}
}
