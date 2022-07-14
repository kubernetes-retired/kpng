// +build !windows

package userspacelin

import "golang.org/x/sys/unix"

func setRLimit(limit uint64) error {
	return unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Max: limit, Cur: limit})
}
