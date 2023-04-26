/*
Copyright 2022 The Kubernetes Authors.

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

package healthchecks

import (
	"net"
	"net/netip"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/diffstore"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

type backend struct {
	cfg       localsink.Config
	ip        string
	statuses  *diffstore.Store[string, *leaf]
	instances map[string]*hcInstance
}

func init() {
	backendcmd.Register("to-healthchecks", func() backendcmd.Cmd {
		return &backend{
			statuses:  diffstore.NewAnyStore[string](isSameStatus),
			instances: map[string]*hcInstance{},
		}
	})
}

func (b *backend) BindFlags(flags *pflag.FlagSet) {
	b.cfg.BindFlags(flags)

	flags.StringVar(&b.ip, "ip", "", "healcheck listeners' IP (or IP CIDR if '/' is included)")
}

func (b *backend) Sink() localsink.Sink {
	sink := fullstate.New(&b.cfg)
	sink.Callback = b.Callback

	// handle the IP parameter
	if strings.ContainsRune(b.ip, '/') {
		prefix, err := netip.ParsePrefix(b.ip)
		if err != nil {
			klog.Fatal("failed to parse ip argument: ", err)
		}

		addrs, err := net.InterfaceAddrs()
		if err != nil {
			klog.Fatal("failed to get interface IPs: ", err)
		}

		for _, addr := range addrs {
			addr, err := netip.ParsePrefix(addr.String())
			if err != nil {
				klog.Fatalf("invalid interface addr %s: %v", addr, err)
			}

			ip := addr.Addr()

			if prefix.Contains(ip) {
				b.ip = ip.String()
			}
		}
	}

	if b.ip != "" {
		// check the IP and wrap as needed (IPv6 a::f -> [a::f])
		ip, err := netip.ParseAddr(b.ip)
		if err != nil {
			klog.Fatalf("invalid bind IP %q: %v", b.ip, err)
		}

		if ip.Is6() {
			b.ip = "[" + ip.String() + "]"
		}

		klog.Info("healthchecks will be served on ", b.ip)
	}

	return sink
}

func isSameStatus(a, b hcStatus) bool { return a.count == b.count }
