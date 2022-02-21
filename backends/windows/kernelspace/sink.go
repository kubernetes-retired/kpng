//go:build windows
// +build windows

/*
Copyright 2017-2022 The Kubernetes Authors.

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

package kernelspace

import (
	"net"
	"os"
	"time"

	"github.com/spf13/pflag"

	"k8s.io/client-go/tools/events"
	// 	"k8s.io/kubernetes/pkg/proxy/apis/config"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
	"sigs.k8s.io/kpng/client/serviceevents"

	klog "k8s.io/klog/v2"
)

type Backend struct {
	localsink.Config
}

var (
	_             decoder.Interface = &Backend{}
	proxier       Provider
	proxierState  Proxier
	flag          = &pflag.FlagSet{}
	minSyncPeriod time.Duration
	syncPeriod    time.Duration

	// TODO JAy add this back , avoiding pkg/proxy imports
	healthzServer healthcheck.ProxierHealthUpdater
	recorder      events.EventRecorder

	masqueradeAll = flag.Bool(
		"masquerade-all",
		false,
		"Set this flag to set the masq rule for all traffic")

	masqueradeBit = flag.Int(
		"masquerade-bit",
		14,
		"iptablesMasqueradeBit is the bit of the iptables fwmark"+
			" space to mark for SNAT Values must be within the range [0, 31]")

	defaultHostname, _ = os.Hostname()
	hostname           = flag.String(
		"hostname",
		defaultHostname,
		"hostname")

	clusterCIDR = flag.String(
		"",
		"0.0.0.0/0",
		"cluster IPs CIDR")

	nodeIP          net.IP
	winkernelConfig KubeProxyWinkernelConfiguration
)

/* BindFlags will bind the flags */
func BindFlags(flags *pflag.FlagSet) {
	flags.AddFlagSet(flag)
}

/* init will do all initialization for backend registration */
func init() {
	backendcmd.Register(
		"to-winkernel",
		func() backendcmd.Cmd { return New() })
}

func New() *Backend {
	return &Backend{}
}

func (s *Backend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(serviceevents.Wrap(s)))
}

func (s *Backend) DeleteEndpoint(namespace, serviceName, key string) {
	proxierState.serviceChanges.Delete(namespace, serviceName)

}

func (s *Backend) SetService(svc *localnetv1.Service) {
	proxierState.serviceChanges.Update(svc)

}

func (s *Backend) DeleteService(namespace, name string) {
	proxierState.BackendDeleteService(namespace, name)
}

func (s *Backend) SetEndpoint(
	namespace,
	serviceName,
	key string,
	endpoint *localnetv1.Endpoint) {
}

func (s *Backend) Reset() {
	/* noop */
}

func (s *Backend) Setup() {
	var err error

	flag.DurationVar(
		&syncPeriod,
		"sync-period-duration",
		15*time.Second,
		"sync period duration")

	klog.V(0).InfoS("Using Windows Kernel Proxier.")

	//proxyMode := getProxyMode(string(config.Mode), WindowsKernelCompatTester{})
	//dualStackMode := getDualStackMode(config.Winkernel.NetworkName, DualStackCompatTester{})
	//_ = dualStackMode
	//_ = proxyMode

	proxier, err = NewProxier(
		syncPeriod,
		minSyncPeriod,
		*masqueradeAll,
		*masqueradeBit,
		*clusterCIDR,
		*hostname,
		nodeIP,
		recorder,
		healthzServer,
		winkernelConfig)

	if err != nil {
		klog.Fatal(err)
	}

}

func (s *Backend) Sync() {
	//proxier.Sync()
}

func (s *Backend) WaitRequest() (nodeName string, err error) {
	name, _ := os.Hostname()
	return name, nil
}
