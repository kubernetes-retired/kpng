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
	"os"
	"time"

	"github.com/spf13/pflag"

	"k8s.io/client-go/tools/events"
	klog "k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/backendcmd"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
	"sigs.k8s.io/kpng/client/serviceevents"
)

type Backend struct {
	cfg localsink.Config
}

var (
	_ decoder.Interface = &Backend{}
	//proxier       Provider
	//proxierState  Proxier
	proxier       *Proxier
	flag          = &pflag.FlagSet{}
	minSyncPeriod time.Duration
	syncPeriod    time.Duration

	// TODO JAy add this back , avoiding pkg/proxy imports
	// healthzServer healthcheck.ProxierHealthUpdater
	recorder events.EventRecorder

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

	// defaulting to the sig-windows-dev-tools value...
	clusterCIDR = flag.String(
		"cluster-cidr",
		"100.244.0.0/24",
		"cluster IPs CIDR")

	// defaulting to the sig-windows-dev-tools value, should be 127.0.0.1?
	nodeip = flag.String(
		"nodeip",
		"10.20.30.11",
		"cluster IPs CIDR")

	// defaulting to the sig-windows-dev-tools value ...
	sourceVip = flag.String(
		"source-vip",
		"100.244.206.65",
		"Source VIP")

	enableDSR = flag.Bool(
		"enable-dsr",
		false,
		"Set this flag to enable DSR")

	winkernelConfig KubeProxyWinkernelConfiguration
)

/* BindFlags will bind the flags */
func BindFlags(flags *pflag.FlagSet) {
	flags.AddFlagSet(flag)
}

func (b *Backend) BindFlags(flags *pflag.FlagSet) {
	b.cfg.BindFlags(flags)
	BindFlags(flags)
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
	proxier.endpointsChanges.EndpointUpdate(namespace, serviceName, key, nil)

}

func (s *Backend) SetService(svc *localv1.Service) {
	klog.V(0).InfoS("SetService -> %v", svc)
	proxier.serviceChanges.Update(svc)

}

func (s *Backend) DeleteService(namespace, name string) {
	proxier.serviceChanges.Delete(namespace, name)
}

func (s *Backend) SetEndpoint(
	namespace,
	serviceName,
	key string,
	endpoint *localv1.Endpoint) {

	proxier.endpointsChanges.EndpointUpdate(namespace, serviceName, key, endpoint)
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

	klog.Info("Starting Windows Kernel Proxier.")
	klog.InfoS("  Cluster CIDR", "clusterCIDR", *clusterCIDR)
	klog.InfoS("  Enable DSR", "enableDSR", *enableDSR)
	klog.InfoS("  Masquerade all traffic", "masqueradeAll", *masqueradeAll)
	klog.InfoS("  Masquerade bit", "masqueradeBit", *masqueradeBit)
	klog.InfoS("  Node ip", "nodeip", *nodeip)
	klog.InfoS("  Source VIP", "sourceVip", *sourceVip)

	//proxyMode := getProxyMode(string(config.Mode), WindowsKernelCompatTester{})
	//dualStackMode := getDualStackMode(config.Winkernel.NetworkName, DualStackCompatTester{})
	//_ = dualStackMode
	//_ = proxyMode

	winkernelConfig.EnableDSR = *enableDSR
	winkernelConfig.NetworkName = "" // remove from config? proxier gets network name from KUBE_NETWORK env var
	winkernelConfig.SourceVip = *sourceVip

	proxier, err = NewProxier(
		syncPeriod,
		minSyncPeriod,
		*masqueradeAll,
		*masqueradeBit,
		*clusterCIDR,
		*hostname, // should this be nodeName?
		netutils.ParseIPSloppy(*nodeip),
		recorder,
		winkernelConfig)

	if err != nil {
		klog.ErrorS(err, "Failed to create an instance of NewProxier")
		panic("could not initialize proxier")
	}
	go proxier.SyncLoop()

}

func (s *Backend) Sync() {
	klog.V(0).InfoS("backend.Sync()")
	proxier.setInitialized(true)
	proxier.Sync()
}

func (s *Backend) WaitRequest() (nodeName string, err error) {
	klog.V(0).InfoS("wait request")
	name, _ := os.Hostname()
	return name, nil
}
