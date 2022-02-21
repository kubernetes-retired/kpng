//go:build windows
// +build windows

package storecmds

import (
	_ "sigs.k8s.io/kpng/backends/windows/userspace"
        _ "sigs.k8s.io/kpng/backends/windows/kernelspace"
)
