/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
