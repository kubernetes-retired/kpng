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

package winkernel

import (
	"fmt"
	"net"
	"time"

	"github.com/Microsoft/hcsshim/hcn"
	v1 "k8s.io/api/core/v1"
	apiutil "k8s.io/apimachinery/pkg/util/net"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	kubefeatures "k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/proxy"
	"k8s.io/kubernetes/pkg/proxy/apis/config"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"k8s.io/kubernetes/pkg/util/async"
	netutils "k8s.io/utils/net"
)

// NewProxier returns a new Proxier
func NewProxier(
	syncPeriod time.Duration,
	minSyncPeriod time.Duration,
	masqueradeAll bool,
	masqueradeBit int,
	clusterCIDR string,
	hostname string,
	nodeIP net.IP,
	recorder events.EventRecorder,
	healthzServer healthcheck.ProxierHealthUpdater,
	config config.KubeProxyWinkernelConfiguration,
) (*Proxier, error) {
	masqueradeValue := 1 << uint(masqueradeBit)
	masqueradeMark := fmt.Sprintf("%#08x/%#08x", masqueradeValue, masqueradeValue)

	if nodeIP == nil {
		klog.InfoS("Invalid nodeIP, initializing kube-proxy with 127.0.0.1 as nodeIP")
		nodeIP = netutils.ParseIPSloppy("127.0.0.1")
	}

	if len(clusterCIDR) == 0 {
		klog.InfoS("ClusterCIDR not specified, unable to distinguish between internal and external traffic")
	}

	serviceHealthServer := healthcheck.NewServiceHealthServer(
		hostname,
		recorder) /* windows listen to all node addresses */

	hns, supportedFeatures := newHostNetworkService()
	hnsNetworkName, err := getNetworkName(config.NetworkName)
	if err != nil {
		return nil, err
	}

	klog.V(3).InfoS("Cleaning up old HNS policy lists")
	deleteAllHnsLoadBalancerPolicy()

	// Get HNS network information
	hnsNetworkInfo, err := getNetworkInfo(hns, hnsNetworkName)
	if err != nil {
		return nil, err
	}

	// Network could have been detected before Remote Subnet Routes are applied or ManagementIP is updated
	// Sleep and update the network to include new information
	if isOverlay(hnsNetworkInfo) {
		time.Sleep(10 * time.Second)
		hnsNetworkInfo, err = hns.getNetworkByName(hnsNetworkName)
		if err != nil {
			return nil, fmt.Errorf("could not find HNS network %s", hnsNetworkName)
		}
	}

	klog.V(1).InfoS("Hns Network loaded", "hnsNetworkInfo", hnsNetworkInfo)
	isDSR := config.EnableDSR
	if isDSR && !utilfeature.DefaultFeatureGate.Enabled(kubefeatures.WinDSR) {
		return nil, fmt.Errorf("WinDSR feature gate not enabled")
	}
	err = hcn.DSRSupported()
	if isDSR && err != nil {
		return nil, err
	}

	var sourceVip string
	var hostMac string
	if isOverlay(hnsNetworkInfo) {
		if !utilfeature.DefaultFeatureGate.Enabled(kubefeatures.WinOverlay) {
			return nil, fmt.Errorf("WinOverlay feature gate not enabled")
		}
		err = hcn.RemoteSubnetSupported()
		if err != nil {
			return nil, err
		}
		sourceVip = config.SourceVip
		if len(sourceVip) == 0 {
			return nil, fmt.Errorf("source-vip flag not set")
		}

		if nodeIP.IsUnspecified() {
			// attempt to get the correct ip address
			klog.V(2).InfoS("Node ip was unspecified, attempting to find node ip")
			nodeIP, err = apiutil.ResolveBindAddress(nodeIP)
			if err != nil {
				klog.InfoS("Failed to find an ip. You may need set the --bind-address flag", "err", err)
			}
		}

		interfaces, _ := net.Interfaces() //TODO create interfaces
		for _, inter := range interfaces {
			addresses, _ := inter.Addrs()
			for _, addr := range addresses {
				addrIP, _, _ := netutils.ParseCIDRSloppy(addr.String())
				if addrIP.String() == nodeIP.String() {
					klog.V(2).InfoS("Record Host MAC address", "addr", inter.HardwareAddr)
					hostMac = inter.HardwareAddr.String()
				}
			}
		}
		if len(hostMac) == 0 {
			return nil, fmt.Errorf("could not find host mac address for %s", nodeIP)
		}
	}

	isIPv6 := netutils.IsIPv6(nodeIP)
	Proxier := &Proxier{
		endPointsRefCount:   make(endPointsReferenceCountMap),
		serviceMap:          make(proxy.ServiceMap),
		endpointsMap:        make(proxy.EndpointsMap),
		masqueradeAll:       masqueradeAll,
		masqueradeMark:      masqueradeMark,
		clusterCIDR:         clusterCIDR,
		hostname:            hostname,
		nodeIP:              nodeIP,
		recorder:            recorder,
		serviceHealthServer: serviceHealthServer,
		healthzServer:       healthzServer,
		hns:                 hns,
		network:             *hnsNetworkInfo,
		sourceVip:           sourceVip,
		hostMac:             hostMac,
		isDSR:               isDSR,
		supportedFeatures:   supportedFeatures,
		isIPv6Mode:          isIPv6,
	}

	ipFamily := v1.IPv4Protocol
	if isIPv6 {
		ipFamily = v1.IPv6Protocol
	}
	serviceChanges := proxy.NewServiceChangeTracker(Proxier.newServiceInfo, ipFamily, recorder, Proxier.serviceMapChange)
	endPointChangeTracker := proxy.NewEndpointChangeTracker(hostname, Proxier.newEndpointInfo, ipFamily, recorder, Proxier.endpointsMapChange)
	Proxier.endpointsChanges = endPointChangeTracker
	Proxier.serviceChanges = serviceChanges

	burstSyncs := 2
	klog.V(3).InfoS("Record sync param", "minSyncPeriod", minSyncPeriod, "syncPeriod", syncPeriod, "burstSyncs", burstSyncs)
	Proxier.syncRunner = async.NewBoundedFrequencyRunner("sync-runner", Proxier.syncProxyRules, minSyncPeriod, syncPeriod, burstSyncs)
	return Proxier, nil
}
