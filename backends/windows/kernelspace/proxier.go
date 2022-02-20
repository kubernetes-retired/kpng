//go:build windows
//+build windows

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
	"fmt"
	"github.com/Microsoft/hcsshim/hcn"
	v1 "k8s.io/api/core/v1"
	apiutil "k8s.io/apimachinery/pkg/util/net"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/events"
	klog "k8s.io/klog/v2"
	kubefeatures "k8s.io/kubernetes/pkg/features"
//	"k8s.io/kubernetes/pkg/proxy/apis/config"
//	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"k8s.io/kubernetes/pkg/util/async"
	netutils "k8s.io/utils/net"
	"net"
	"time"
)

// NewProxier returns a new Proxier
func NewProxier(
	syncPeriod time.Duration,    //
	minSyncPeriod time.Duration, //
	masqueradeAll bool,
	masqueradeBit int,
	clusterCIDR string,
	hostname string,
	nodeIP net.IP,
	recorder events.EventRecorder,                  // ignore
	//healthzServer healthcheck.ProxierHealthUpdater, // ignore
	config KubeProxyWinkernelConfiguration,
) (*Proxier, error) {

	// ** Why do we have a masquerade bit ? and what is this 1 << uint... doing
	masqueradeValue := 1 << uint(masqueradeBit)
	masqueradeMark := fmt.Sprintf("%#08x/%#08x", masqueradeValue, masqueradeValue)

	if nodeIP == nil {
		klog.InfoS("Invalid nodeIP, initializing kube-proxy with 127.0.0.1 as nodeIP")
		nodeIP = netutils.ParseIPSloppy("127.0.0.1")
	}

	if len(clusterCIDR) == 0 {
		klog.InfoS("ClusterCIDR not specified, unable to distinguish between internal and external traffic")
	}


	// ** not worrying about svc>HealthServer but do we need it later?
	//serviceHealthServer := healthcheck.NewServiceHealthServer(
	//	hostname,
	//	recorder, []string{}) /* windows listen to all node addresses */

	// get a empty HNS network object, that we'll use to make system calls to either h1 or h2.
	// this will introspect the underlying kernel.
	hns, supportedFeatures := newHostNetworkService()
	// --network-name <-- this is often passed in by calico
	hnsNetworkName, err := getNetworkName(config.NetworkName)
	if err != nil {
		return nil, err
	}

	klog.V(3).InfoS("Cleaning up old HNS policy lists")

	// Some kind of cleanup step, we dont fully understand it yet, but the API looks
	// clumsy...should ask danny about this... we should use the hns object probably for this?
	deleteAllHnsLoadBalancerPolicy()

	// Get HNS network information, (name, id, *** networkType *** remoteSubnets)
	// One possible networkType == NETWORK_TYPE_OVERLAY
	//		What other types ? others CNIs like AKS / EKS dont overlay ? ... maybe if its EKS its bridge
	// 		and not overlay or something, worth spending a few hours to analyze these...
	hnsNetworkInfo, err := getNetworkInfo(hns, hnsNetworkName)
	if err != nil {
		return nil, err
	}

	// Network could have been detected ...
	// - BEFORE Remote Subnet Routes were applied
	// - BEFORE ManagementIP is updated
	// So.. sleep just for a second...
	if isOverlay(hnsNetworkInfo) {
		// ... Why do we do this sleep ?
		time.Sleep(10 * time.Second)

		// Is this just to REFRESH the value of this field ?? ? ? ? ? ? ?????
		hnsNetworkInfo, err = hns.getNetworkByName(hnsNetworkName)
		if err != nil {
			return nil, fmt.Errorf("could not find HNS network %s", hnsNetworkName)
		}
	}

	klog.V(1).InfoS("Hns Network loaded", "hnsNetworkInfo", hnsNetworkInfo)

	// Direct Server return is a optimization , we can ignore it for now...
	isDSR := config.EnableDSR
	if isDSR && !utilfeature.DefaultFeatureGate.Enabled(kubefeatures.WinDSR) {
		return nil, fmt.Errorf("WinDSR feature gate not enabled")
	}
	err = hcn.DSRSupported()
	if isDSR && err != nil {
		return nil, err
	}

	// Why do we need VIPs?

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
		serviceMap:          make(ServiceMap),
		endpointsMap:        make(EndpointsMap),
		masqueradeAll:       masqueradeAll,
		masqueradeMark:      masqueradeMark,
		clusterCIDR:         clusterCIDR,
		hostname:            hostname,
		nodeIP:              nodeIP,
		recorder:            recorder,
		//serviceHealthServer: serviceHealthServer,
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

	/**
	func(*localnetv1.PortMapping, *localnetv1.Service, *BaseServiceInfo)
		(port *v1.ServicePort,    service *v1.Service, baseInfo *BaseServiceInfo)
	- (string,
		func(baseInfo *proxy.BaseEndpointInfo)
			proxy.Endpoint,
			"k8s.io/api/core/v1".IPFamily,
	    	events.EventRecorder
		)
	- vnet1 portmapping, vnet1 service, BaseServiceInfo
	*/
	serviceChanges := NewServiceChangeTracker(Proxier.newServiceInfo, ipFamily, recorder)
	endPointChangeTracker := NewEndpointChangeTracker(hostname, ipFamily, recorder)
	Proxier.endpointsChanges = endPointChangeTracker
	Proxier.serviceChanges = serviceChanges

	burstSyncs := 2
	klog.V(3).InfoS("Record sync param", "minSyncPeriod", minSyncPeriod, "syncPeriod", syncPeriod, "burstSyncs", burstSyncs)
	Proxier.syncRunner = async.NewBoundedFrequencyRunner("sync-runner", Proxier.syncProxyRules, minSyncPeriod, syncPeriod, burstSyncs)
	return Proxier, nil
}
