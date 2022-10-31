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
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Microsoft/hcsshim/hcn"

	v1 "k8s.io/api/core/v1"
	apiutil "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/util/async"
	netutils "k8s.io/utils/net"

	"sigs.k8s.io/kpng/api/localv1"
)

// Provider is a proxy interface enforcing services and windowsEndpoint methods
// implementations
type Provider interface {
	// OnEndpointsAdd is called whenever creation of new windowsEndpoint object
	// is observed.
	OnEndpointsAdd(ep *localv1.Endpoint, svc *localv1.Service)
	// OnEndpointsDelete is called whenever deletion of an existing windowsEndpoint
	// object is observed.
	OnEndpointsDelete(ep *localv1.Endpoint, svc *localv1.Service)
	// OnEndpointsSynced is called once all the initial event handlers were
	// called and the state is fully propagated to local cache.
	OnEndpointsSynced()
	// OnServiceAdd is called whenever creation of new service object
	// is observed.
	OnServiceAdd(service *localv1.Service)
	// OnServiceUpdate is called whenever modification of an existing
	// service object is observed.
	OnServiceUpdate(oldService, service *localv1.Service)
	// OnServiceDelete is called whenever deletion of an existing service
	// object is observed.
	OnServiceDelete(service *localv1.Service)
	// OnServiceSynced is called once all the initial event handlers were
	// called and the state is fully propagated to local cache.
	OnServiceSynced()

	// Sync immediately synchronizes the Provider's current state to proxy rules.
	Sync()
	// SyncLoop runs periodic work.
	// This is expected to run as a goroutine or as the main loop of the app.
	// It does not return.
	SyncLoop()
}

// Proxier (windows/kernelspace/Proxier) is copied impl of windows kernel based proxy for connections between a localhost:lport
// and services that provide the actual backends.
type Proxier struct {
	// TODO(imroc): implement node handler for winkernel proxier.
	//proxyconfig.NoopNodeHandler
	// endpointsChanges and serviceChanges contains all changes to windowsEndpoint and
	// services that happened since policies were synced. For a single object,
	// changes are accumulated, i.e. previous is state from before all of them,
	// current is state after applying all of those.
	endpointsChanges  *EndpointChangeTracker
	serviceChanges    *ServiceChangeTracker
	endPointsRefCount endPointsReferenceCountMap
	mu                sync.Mutex // protects the following fields
	serviceMap        ServicesSnapshot
	endpointsMap      EndpointsMap
	// endpointSlicesSynced and servicesSynced are set to true when corresponding
	// objects are synced after startup. This is used to avoid updating hns policies
	// with some partial data after kube-proxy restart.
	endpointSlicesSynced bool
	servicesSynced       bool
	isIPv6Mode           bool
	initialized          int32
	syncRunner           *async.BoundedFrequencyRunner // governs calls to syncProxyRules
	// These are effectively const and do not need the mutex to be held.
	masqueradeAll  bool
	masqueradeMark string
	clusterCIDR    string
	hostname       string
	nodeIP         net.IP
	recorder       events.EventRecorder

	// Since converting probabilities (floats) to strings is expensive
	// and we are using only probabilities in the format of 1/n, we are
	// precomputing some number of those and cache for future reuse.
	precomputedProbabilities []string

	hns               HCNUtils
	network           hnsNetworkInfo
	sourceVip         string
	hostMac           string
	isDSR             bool
	supportedFeatures hcn.SupportedFeatures
}

// BaseEndpointInfo contains base information that defines an endpoint.
// This could be used directly by proxier while processing endpoints,
// or can be used for constructing a more specific EndpointInfo struct
// defined by the proxier if needed.
type BaseEndpointInfo struct {
	Endpoint string // TODO: should be an endpointString type
	// IsLocal indicates whether the endpoint is running in same host as kube-proxy.
	IsLocal bool
	// ZoneHints represent the zone hints for the endpoint. This is based on
	// endpoint.hints.forZones[*].name in the EndpointSlice API.
	ZoneHints sets.String
	// Ready indicates whether this endpoint is ready and NOT terminating.
	// For pods, this is true if a pod has a ready status and a nil deletion timestamp.
	// This is only set when watching EndpointSlices. If using Endpoints, this is always
	// true since only ready endpoints are read from Endpoints.
	// TODO: Ready can be inferred from Serving and Terminating below when enabled by default.
	Ready bool
	// Serving indicates whether this endpoint is ready regardless of its terminating state.
	// For pods this is true if it has a ready status regardless of its deletion timestamp.
	// This is only set when watching EndpointSlices. If using Endpoints, this is always
	// true since only ready endpoints are read from Endpoints.
	Serving bool
	// Terminating indicates whether this endpoint is terminating.
	// For pods this is true if it has a non-nil deletion timestamp.
	// This is only set when watching EndpointSlices. If using Endpoints, this is always
	// false since terminating endpoints are always excluded from Endpoints.
	Terminating bool
	// NodeName is the name of the node this endpoint belongs to
	NodeName string
	// Zone is the name of the zone this endpoint belongs to
	Zone string
}

func (ep *BaseEndpointInfo) GetTopology() map[string]string {
	return map[string]string{}
}

var _ Endpoint = &BaseEndpointInfo{}

// String is part of proxy.Endpoint interface.
func (info *BaseEndpointInfo) String() string {
	return info.Endpoint
}

// GetIsLocal is part of proxy.Endpoint interface.
func (info *BaseEndpointInfo) GetIsLocal() bool {
	return info.IsLocal
}

// IsReady returns true if an endpoint is ready and not terminating.
func (info *BaseEndpointInfo) IsReady() bool {
	return info.Ready
}

// IsServing returns true if an endpoint is ready, regardless of if the
// endpoint is terminating.
func (info *BaseEndpointInfo) IsServing() bool {
	return info.Serving
}

// IsTerminating returns true if an endpoint is terminating. For pods,
// that is any pod with a deletion timestamp.
func (info *BaseEndpointInfo) IsTerminating() bool {
	return info.Terminating
}

// GetZoneHints returns the zone hint for the endpoint.
func (info *BaseEndpointInfo) GetZoneHints() sets.String {
	return info.ZoneHints
}

// IP returns just the IP part of the endpoint, it's a part of proxy.Endpoint interface.
func (info *BaseEndpointInfo) IP() string {
	return IPPart(info.Endpoint)
}

// Port returns just the Port part of the endpoint.
func (info *BaseEndpointInfo) Port() (int, error) {
	return PortPart(info.Endpoint)
}

// Equal is part of proxy.Endpoint interface.
func (info *BaseEndpointInfo) Equal(other Endpoint) bool {
	return info.String() == other.String() &&
		info.GetIsLocal() == other.GetIsLocal() &&
		info.IsReady() == other.IsReady()
}

// GetNodeName returns the NodeName for this endpoint.
func (info *BaseEndpointInfo) GetNodeName() string {
	return info.NodeName
}

// GetZone returns the Zone for this endpoint.
func (info *BaseEndpointInfo) GetZone() string {
	return info.Zone
}

type endPointsReferenceCountMap map[string]*uint16

func (refCountMap endPointsReferenceCountMap) getRefCount(hnsID string) *uint16 {
	refCount, exists := refCountMap[hnsID]
	if !exists {
		refCountMap[hnsID] = new(uint16)
		refCount = refCountMap[hnsID]
	}
	return refCount
}

func newHostNetworkService() (HCNUtils, hcn.SupportedFeatures) {
	var h HCNUtils
	supportedFeatures := hcn.GetSupportedFeatures()
	if supportedFeatures.Api.V2 {
		h = hcnutils{&ihcn{}}
	}

	return h, supportedFeatures
}

func getNetworkName(hnsNetworkName string) (string, error) {
	if len(hnsNetworkName) == 0 {
		klog.V(3).InfoS("Flag --network-name not set, checking environment variable")
		hnsNetworkName = os.Getenv("KUBE_NETWORK")
		if len(hnsNetworkName) == 0 {
			return "", fmt.Errorf("Environment variable KUBE_NETWORK and network-flag not initialized")
		}
	}
	return hnsNetworkName, nil
}

func getNetworkInfo(hns HCNUtils, hnsNetworkName string) (*hnsNetworkInfo, error) {
	hnsNetworkInfo, err := hns.getNetworkByName(hnsNetworkName)
	for err != nil {
		klog.ErrorS(err, "Unable to find HNS Network specified, please check network name and CNI deployment", "hnsNetworkName", hnsNetworkName)
		time.Sleep(1 * time.Second)
		hnsNetworkInfo, err = hns.getNetworkByName(hnsNetworkName)
	}
	return hnsNetworkInfo, err
}

// NewProxier returns a new Proxier
func NewProxier(
	syncPeriod time.Duration,    //
	minSyncPeriod time.Duration, //
	masqueradeAll bool,
	masqueradeBit int,
	clusterCIDR string,
	hostname string,
	nodeIP net.IP,
	recorder events.EventRecorder, // ignore
	config KubeProxyWinkernelConfiguration,
) (*Proxier, error) {

	// ** Why do we have a masquerade bit ? and what is this 1 << uint... doing
	masqueradeValue := 1 << uint(masqueradeBit)
	masqueradeMark := fmt.Sprintf("%#08x/%#08x", masqueradeValue, masqueradeValue)

	if nodeIP == nil {
		klog.Warning("Invalid nodeIP, initializing kube-proxy with 10.20.30.11 as nodeIP")
		nodeIP = netutils.ParseIPSloppy("10.20.30.11")
	}

	if len(clusterCIDR) == 0 {
		klog.Warning("ClusterCIDR not specified, unable to distinguish between internal and external traffic")
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

	isDSR := config.EnableDSR
	err = hcn.DSRSupported()
	if isDSR && err != nil {
		return nil, err
	}

	klog.InfoS("Enable DSR?", "isDSR", winkernelConfig.EnableDSR)

	// Why do we need VIPs?

	//var sourceVip string
	var hostMac string
	if isOverlay(hnsNetworkInfo) {
		if !true /*utilfeature.DefaultFeatureGate.Enabled(kubefeatures.WinOverlay)*/ {
			return nil, fmt.Errorf("WinOverlay feature gate not enabled")
		}
		err = hcn.RemoteSubnetSupported()
		if err != nil {
			return nil, err
		}
		sourceVip := config.SourceVip
		if len(sourceVip) == 0 {
			return nil, fmt.Errorf("source-vip flag not set and is required for overlay networking")
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
				if addrIP.String() == nodeIP.String() && inter.HardwareAddr != nil {
					klog.V(2).InfoS("Record Host MAC address", "addr", inter.HardwareAddr)
					hostMac = inter.HardwareAddr.String()
				}
			}
		}
		// TODO: Add Back the flag
		//if len(hostMac) == 0 {
		//	return nil, fmt.Errorf("could not find host mac address for %s", nodeIP)
		//}
	}

	isIPv6 := netutils.IsIPv6(nodeIP)
	myProxier := &Proxier{
		endPointsRefCount: make(endPointsReferenceCountMap),
		serviceMap:        make(ServicesSnapshot),
		endpointsMap:      make(EndpointsMap),
		masqueradeAll:     masqueradeAll,
		masqueradeMark:    masqueradeMark,
		clusterCIDR:       clusterCIDR,
		hostname:          hostname,
		nodeIP:            nodeIP,
		recorder:          recorder,
		hns:               hns,
		network:           *hnsNetworkInfo,
		sourceVip:         *sourceVip,
		hostMac:           hostMac,
		isDSR:             isDSR,
		supportedFeatures: supportedFeatures,
		isIPv6Mode:        isIPv6,
	}

	ipFamily := v1.IPv4Protocol
	if isIPv6 {
		ipFamily = v1.IPv6Protocol
	}

	/**
	func(*kpng.PortMapping, *kpng.Service, *BaseServiceInfo)
		(port *v1.ServicePort,    service *v1.Service, baseInfo *BaseServiceInfo)
	- (string,
		func(baseInfo *proxy.BaseEndpointInfo)
			proxy.Endpoint,
			"k8s.io/api/core/v1".IPFamily,
	    	events.EventRecorder
		)
	- vnet1 portmapping, vnet1 service, BaseServiceInfo
	*/
	serviceChanges := NewServiceChangeTracker(myProxier.newServiceInfo, ipFamily, recorder)
	endPointChangeTracker := NewEndpointChangeTracker(hostname, ipFamily, recorder)
	myProxier.endpointsChanges = endPointChangeTracker
	myProxier.serviceChanges = serviceChanges

	burstSyncs := 2
	klog.V(3).InfoS("Record sync param", "minSyncPeriod", minSyncPeriod, "syncPeriod", syncPeriod, "burstSyncs", burstSyncs)
	myProxier.syncRunner = async.NewBoundedFrequencyRunner("sync-runner", myProxier.syncProxyRules, minSyncPeriod, syncPeriod, burstSyncs)
	return myProxier, nil
}

// Sync is called to synchronize the Proxier state to hns as soon as possible.
func (proxier *Proxier) Sync() {
	//	if Proxier.healthzServer != nil {
	//		Proxier.healthzServer.QueuedUpdate()
	//	}

	// TODO commenting out metrics, Jay to fix , figure out how to  copy these later, avoiding pkg/proxy imports
	// metrics.SyncProxyRulesLastQueuedTimestamp.SetToCurrentTime()

	klog.V(0).InfoS("proxier_sync.Sync ->")
	proxier.syncRunner.Run()
}

// SyncLoop runs periodic work.  This is expected to run as a goroutine or as the main loop of the app.  It does not return.
func (proxier *Proxier) SyncLoop() {
	// Update healthz timestamp at beginning in case Sync() never succeeds.
	//	if proxier.healthzServer != nil {
	//		proxier.healthzServer.Updated()
	//	}
	// synthesize "last change queued" time as the informers are syncing.
	//	metrics.SyncProxyRulesLastQueuedTimestamp.SetToCurrentTime()
	proxier.syncRunner.Loop(wait.NeverStop)
}

func (proxier *Proxier) isInitialized() bool {
	return atomic.LoadInt32(&proxier.initialized) > 0
}

func (proxier *Proxier) cleanupAllPolicies() {
	for svcName, svcPortMap := range proxier.serviceMap {
		for _, svc := range svcPortMap {

			svcInfo, ok := svc.(*serviceInfo)
			if !ok {
				klog.ErrorS(nil, "Failed to cast serviceInfo", "serviceName", svcName)
				continue
			}
			endpoints := proxier.endpointsMap[svcName]
			if endpoints != nil {
				for _, e := range *endpoints {
					svcInfo.cleanupAllPolicies(e)
				}
			}
		}
	}
}

type loadBalancerInfo struct {
	hnsID string
}

type loadBalancerIdentifier struct {
	protocol       uint16
	internalPort   uint16
	externalPort   uint16
	vip            string
	endpointsCount int
}

type loadBalancerFlags struct {
	isILB           bool
	isDSR           bool
	localRoutedVIP  bool
	useMUX          bool
	preserveDIP     bool
	sessionAffinity bool
	isIPv6          bool
}

type hnsNetworkInfo struct {
	name          string
	id            string
	networkType   string
	remoteSubnets []*remoteSubnetInfo
}

type remoteSubnetInfo struct {
	destinationPrefix string
	isolationID       uint16
	providerAddress   string
	drMacAddress      string
}

const NETWORK_TYPE_OVERLAY = "overlay"
const NETWORK_TYPE_L2BRIDGE = "L2Bridge"

// This is where all of the hns save/restore calls happen.
// assumes Proxier.mu is held
func (proxier *Proxier) syncProxyRules() {
	proxier.mu.Lock()
	defer proxier.mu.Unlock()

	// don't sync rules till we've received services and endpoints
	if !proxier.isInitialized() {
		klog.V(2).InfoS("Not syncing hns until Services and Endpoints have been received from master")
		return
	}

	// Keep track of how long syncs take.
	start := time.Now()
	defer func() {
		//metrics.SyncProxyRulesLatency.Observe(metrics.SinceInSeconds(start))
		klog.V(4).InfoS("Syncing proxy rules complete", "elapsed", time.Since(start))
	}()

	hnsNetworkName := proxier.network.name
	hns := proxier.hns

	prevNetworkID := proxier.network.id
	updatedNetwork, err := hns.getNetworkByName(hnsNetworkName)
	if updatedNetwork == nil || updatedNetwork.id != prevNetworkID || isNetworkNotFoundError(err) {
		klog.InfoS("The HNS network is not present or has changed since the last sync, please check the CNI deployment", "hnsNetworkName", hnsNetworkName)
		proxier.cleanupAllPolicies()
		if updatedNetwork != nil {
			proxier.network = *updatedNetwork
		}
		return
	}

	// We assume that if this was called, we really want to sync them,
	// even if nothing changed in the meantime. In other words, callers are
	// responsible for detecting no-op changes and not calling this function.
	serviceUpdateResult := proxier.serviceMap.Update(proxier.serviceChanges)
	endpointUpdateResult := proxier.endpointsMap.Update(proxier.endpointsChanges)

	staleServices := serviceUpdateResult.UDPStaleClusterIP
	// merge stale services gathered from updateEndpointsMap
	for _, svcPortName := range endpointUpdateResult.StaleServiceNames {
		klog.InfoS("echo %v", svcPortName)
		//if svcInfo, ok := proxier.serviceMap[svcPortName]; ok && svcInfo != nil && svcInfo.Protocol() == v1.ProtocolUDP {
		//	klog.V(2).InfoS("Stale udp service", "servicePortName", svcPortName, "clusterIP", svcInfo.ClusterIP())
		//	staleServices.Insert(svcInfo.ClusterIP().String())
		//}
	}
	// Query HNS for endpoints and load balancers
	queriedEndpoints, err := hns.getAllEndpointsByNetwork(hnsNetworkName)
	if err != nil {
		klog.ErrorS(err, "Querying HNS for endpoints failed")
		return
	}
	if queriedEndpoints == nil {
		klog.V(4).InfoS("No existing endpoints found in HNS")
		queriedEndpoints = make(map[string]*(endpointsInfo))
	}
	queriedLoadBalancers, err := hns.getAllLoadBalancers()
	if queriedLoadBalancers == nil {
		klog.V(4).InfoS("No existing load balancers found in HNS")
		queriedLoadBalancers = make(map[loadBalancerIdentifier]*(loadBalancerInfo))
	}
	if err != nil {
		klog.ErrorS(err, "Querying HNS for load balancers failed")
		return
	}
	if strings.EqualFold(proxier.network.networkType, NETWORK_TYPE_OVERLAY) {
		if _, ok := queriedEndpoints[proxier.sourceVip]; !ok {
			_, err = newSourceVIP(hns, hnsNetworkName, proxier.sourceVip, proxier.hostMac, proxier.nodeIP.String())
			if err != nil {
				klog.ErrorS(err, "Source Vip endpoint creation failed")
				return
			}
		}
	}

	klog.V(3).InfoS("Syncing Policies")

	// Program HNS by adding corresponding policies for each service.
	for svcName, svcPortMap := range proxier.serviceMap {
		for _, svc := range svcPortMap {
			svcInfo, ok := svc.(*serviceInfo)
			if !ok {
				klog.ErrorS(nil, "Failed to cast serviceInfo", "serviceName", svcName)
				continue
			}

			if svcInfo.policyApplied {
				klog.V(4).InfoS("Policy already applied", "serviceInfo", svcInfo)
				continue
			}

			if strings.EqualFold(proxier.network.networkType, NETWORK_TYPE_OVERLAY) {
				serviceVipEndpoint := queriedEndpoints[svcInfo.ClusterIP().String()]
				if serviceVipEndpoint == nil {
					klog.V(4).InfoS("No existing remote endpoint", "IP", svcInfo.ClusterIP())
					hnsEndpoint := &endpointsInfo{
						ip:              svcInfo.ClusterIP().String(),
						isLocal:         false,
						macAddress:      proxier.hostMac,
						providerAddress: proxier.nodeIP.String(),
					}

					newHnsEndpoint, err := hns.createEndpoint(hnsEndpoint, hnsNetworkName)
					if err != nil {
						klog.ErrorS(err, "Remote endpoint creation failed for service VIP")
						continue
					}

					newHnsEndpoint.refCount = proxier.endPointsRefCount.getRefCount(newHnsEndpoint.hnsID)
					*newHnsEndpoint.refCount++
					svcInfo.remoteEndpoint = newHnsEndpoint
					// store newly created endpoints in queriedEndpoints
					queriedEndpoints[newHnsEndpoint.hnsID] = newHnsEndpoint
					queriedEndpoints[newHnsEndpoint.ip] = newHnsEndpoint
				}
			}

			var hnsEndpoints []endpointsInfo
			var hnsLocalEndpoints []endpointsInfo
			klog.V(4).InfoS("Applying Policy", "serviceInfo", svcName)
			// Create Remote endpoints for every endpoint, corresponding to the service
			containsPublicIP := false
			containsNodeIP := false

			endpoints, ok := proxier.endpointsMap[svcName]
			if ok {
				for _, e := range *endpoints {
					ep := &endpointsInfo{
						ip:      e.IPs.First(),
						isLocal: e.Local,
						hns:     proxier.hns,
						ready:   true,
						serving: true, // TODO same as above?
					}

					if !ok {
						klog.ErrorS(nil, "Failed to cast endpointsInfo", "serviceName", svcName)
						continue
					}

					if !ep.IsReady() {
						continue
					}
					var newHnsEndpoint *endpointsInfo
					hnsNetworkName := proxier.network.name
					var err error

					// targetPort is zero if it is specified as a name in port.TargetPort, so the real port should be got from endpoints.
					// Note that hcsshim.AddLoadBalancer() doesn't support endpoints with different ports, so only port from first endpoint is used.
					// TODO(feiskyer): add support of different endpoint ports after hcsshim.AddLoadBalancer() add that.
					if svcInfo.targetPort == 0 {
						svcInfo.targetPort = int(ep.port)
					}
					// There is a bug in Windows Server 2019 that can cause two endpoints to be created with the same IP address, so we need to check using endpoint ID first.
					// TODO: Remove lookup by endpoint ID, and use the IP address only, so we don't need to maintain multiple keys for lookup.
					if len(ep.hnsID) > 0 {
						newHnsEndpoint = queriedEndpoints[ep.hnsID]
					}

					if newHnsEndpoint == nil {
						// First check if an endpoint resource exists for this IP, on the current host
						// A Local endpoint could exist here already
						// A remote endpoint was already created and proxy was restarted
						newHnsEndpoint = queriedEndpoints[ep.IP()]
					}

					if newHnsEndpoint == nil {
						if ep.GetIsLocal() {
							klog.ErrorS(err, "Local endpoint not found: on network", "ip", ep.IP(), "hnsNetworkName", hnsNetworkName)
							continue
						}

						if strings.EqualFold(proxier.network.networkType, NETWORK_TYPE_OVERLAY) {
							klog.InfoS("Updating network to check for new remote subnet policies", "networkName", proxier.network.name)
							networkName := proxier.network.name
							updatedNetwork, err := hns.getNetworkByName(networkName)
							if err != nil {
								klog.ErrorS(err, "Unable to find HNS Network specified, please check network name and CNI deployment", "hnsNetworkName", hnsNetworkName)
								proxier.cleanupAllPolicies()
								return
							}
							proxier.network = *updatedNetwork
							providerAddress := proxier.network.findRemoteSubnetProviderAddress(ep.IP())
							if len(providerAddress) == 0 {
								klog.InfoS("Could not find provider address, assuming it is a public IP", "IP", ep.IP())
								providerAddress = proxier.nodeIP.String()
							}

							hnsEndpoint := &endpointsInfo{
								ip:              ep.ip,
								isLocal:         false,
								macAddress:      conjureMac("02-11", netutils.ParseIPSloppy(ep.ip)),
								providerAddress: providerAddress,
							}

							newHnsEndpoint, err = hns.createEndpoint(hnsEndpoint, hnsNetworkName)
							if err != nil {
								klog.ErrorS(err, "Remote endpoint creation failed", "endpointsInfo", hnsEndpoint)
								continue
							}
						} else {

							hnsEndpoint := &endpointsInfo{
								ip:         ep.ip,
								isLocal:    false,
								macAddress: ep.macAddress,
							}

							newHnsEndpoint, err = hns.createEndpoint(hnsEndpoint, hnsNetworkName)
							if err != nil {
								klog.ErrorS(err, "Remote endpoint creation failed")
								continue
							}
						}
					}
					// For Overlay networks 'SourceVIP' on an Load balancer Policy can either be chosen as
					// a) Source VIP configured on kube-proxy (or)
					// b) Node IP of the current node
					//
					// For L2Bridge network the Source VIP is always the NodeIP of the current node and the same
					// would be configured on kube-proxy as SourceVIP
					//
					// The logic for choosing the SourceVIP in Overlay networks is based on the backend endpoints:
					// a) Endpoints are any IP's outside the cluster ==> Choose NodeIP as the SourceVIP
					// b) Endpoints are IP addresses of a remote node => Choose NodeIP as the SourceVIP
					// c) Everything else (Local POD's, Remote POD's, Node IP of current node) ==> Choose the configured SourceVIP
					if strings.EqualFold(proxier.network.networkType, NETWORK_TYPE_OVERLAY) && !ep.GetIsLocal() {
						providerAddress := proxier.network.findRemoteSubnetProviderAddress(ep.IP())

						isNodeIP := (ep.IP() == providerAddress)
						isPublicIP := (len(providerAddress) == 0)
						klog.InfoS("Endpoint on overlay network", "ip", ep.IP(), "hnsNetworkName", hnsNetworkName, "isNodeIP", isNodeIP, "isPublicIP", isPublicIP)

						containsNodeIP = containsNodeIP || isNodeIP
						containsPublicIP = containsPublicIP || isPublicIP
					}

					// Save the hnsId for reference
					klog.V(1).InfoS("Hns endpoint resource", "endpointsInfo", newHnsEndpoint)

					hnsEndpoints = append(hnsEndpoints, *newHnsEndpoint)
					if newHnsEndpoint.GetIsLocal() {
						hnsLocalEndpoints = append(hnsLocalEndpoints, *newHnsEndpoint)
					} else {
						// We only share the refCounts for remote endpoints
						ep.refCount = proxier.endPointsRefCount.getRefCount(newHnsEndpoint.hnsID)
						*ep.refCount++
					}

					ep.hnsID = newHnsEndpoint.hnsID

					klog.V(3).InfoS("Endpoint resource found", "endpointsInfo", ep)
				}
			}

			klog.V(3).InfoS("Associated endpoints for service", "endpointsInfo", hnsEndpoints, "serviceName", svcName)

			if len(svcInfo.hnsID) > 0 {
				// This should not happen
				klog.InfoS("Load Balancer already exists -- Debug ", "hnsID", svcInfo.hnsID)
			}

			if len(hnsEndpoints) == 0 {
				klog.ErrorS(nil, "Endpoint information not available for service, not applying any policy", "serviceName", svcName)
				continue
			}

			klog.V(4).InfoS("Trying to apply Policies for service", "serviceInfo", svcInfo)
			var hnsLoadBalancer *loadBalancerInfo
			var sourceVip = proxier.sourceVip
			if containsPublicIP || containsNodeIP {
				sourceVip = proxier.nodeIP.String()
			}

			sessionAffinityClientIP := svcInfo.SessionAffinityType() == v1.ServiceAffinityClientIP
			if sessionAffinityClientIP && !proxier.supportedFeatures.SessionAffinity {
				klog.InfoS("Session Affinity is not supported on this version of Windows")
			}

			hnsLoadBalancer, err := hns.getLoadBalancer(
				hnsEndpoints,
				loadBalancerFlags{isDSR: proxier.isDSR, isIPv6: proxier.isIPv6Mode, sessionAffinity: sessionAffinityClientIP},
				sourceVip,
				svcInfo.ClusterIP().String(),
				Enum(svcInfo.Protocol()),
				uint16(svcInfo.targetPort),
				uint16(svcInfo.Port()),
				queriedLoadBalancers,
			)
			if err != nil {
				klog.ErrorS(err, "Policy creation failed")
				continue
			}

			svcInfo.hnsID = hnsLoadBalancer.hnsID
			klog.V(3).InfoS("Hns LoadBalancer resource created for cluster ip resources", "clusterIP", svcInfo.ClusterIP(), "hnsID", hnsLoadBalancer.hnsID)

			// If nodePort is specified, user should be able to use nodeIP:nodePort to reach the backend endpoints
			if svcInfo.NodePort() > 0 {
				// If the preserve-destination service annotation is present, we will disable routing mesh for NodePort.
				// This means that health services can use Node Port without falsely getting results from a different node.
				nodePortEndpoints := hnsEndpoints
				if svcInfo.preserveDIP || svcInfo.localTrafficDSR {
					nodePortEndpoints = hnsLocalEndpoints
				}

				if len(nodePortEndpoints) > 0 {
					hnsLoadBalancer, err := hns.getLoadBalancer(
						nodePortEndpoints,
						loadBalancerFlags{isDSR: svcInfo.localTrafficDSR, localRoutedVIP: true, sessionAffinity: sessionAffinityClientIP, isIPv6: proxier.isIPv6Mode},
						sourceVip,
						"",
						Enum(svcInfo.Protocol()),
						uint16(svcInfo.targetPort),
						uint16(svcInfo.NodePort()),
						queriedLoadBalancers,
					)
					if err != nil {
						klog.ErrorS(err, "Policy creation failed")
						continue
					}

					svcInfo.nodePorthnsID = hnsLoadBalancer.hnsID
					klog.V(3).InfoS("Hns LoadBalancer resource created for nodePort resources", "clusterIP", svcInfo.ClusterIP(), "nodeport", svcInfo.NodePort(), "hnsID", hnsLoadBalancer.hnsID)
				} else {
					klog.V(3).InfoS("Skipped creating Hns LoadBalancer for nodePort resources", "clusterIP", svcInfo.ClusterIP(), "nodeport", svcInfo.NodePort(), "hnsID", hnsLoadBalancer.hnsID)
				}
			}

			// Create a Load Balancer Policy for each external IP
			for _, externalIP := range svcInfo.externalIPs {
				// Disable routing mesh if ExternalTrafficPolicy is set to local
				externalIPEndpoints := hnsEndpoints
				if svcInfo.localTrafficDSR {
					externalIPEndpoints = hnsLocalEndpoints
				}

				if len(externalIPEndpoints) > 0 {
					// Try loading existing policies, if already available
					hnsLoadBalancer, err = hns.getLoadBalancer(
						externalIPEndpoints,
						loadBalancerFlags{isDSR: svcInfo.localTrafficDSR, sessionAffinity: sessionAffinityClientIP, isIPv6: proxier.isIPv6Mode},
						sourceVip,
						externalIP.ip,
						Enum(svcInfo.Protocol()),
						uint16(svcInfo.targetPort),
						uint16(svcInfo.Port()),
						queriedLoadBalancers,
					)
					if err != nil {
						klog.ErrorS(err, "Policy creation failed")
						continue
					}
					externalIP.hnsID = hnsLoadBalancer.hnsID
					klog.V(3).InfoS("Hns LoadBalancer resource created for externalIP resources", "externalIP", externalIP, "hnsID", hnsLoadBalancer.hnsID)
				} else {
					klog.V(3).InfoS("Skipped creating Hns LoadBalancer for externalIP resources", "externalIP", externalIP, "hnsID", hnsLoadBalancer.hnsID)
				}
			}
			// Create a Load Balancer Policy for each loadbalancer ingress
			for _, lbIngressIP := range svcInfo.loadBalancerIngressIPs {
				// Try loading existing policies, if already available
				lbIngressEndpoints := hnsEndpoints
				if svcInfo.preserveDIP || svcInfo.localTrafficDSR {
					lbIngressEndpoints = hnsLocalEndpoints
				}

				if len(lbIngressEndpoints) > 0 {
					hnsLoadBalancer, err := hns.getLoadBalancer(
						lbIngressEndpoints,
						loadBalancerFlags{isDSR: svcInfo.preserveDIP || svcInfo.localTrafficDSR, useMUX: svcInfo.preserveDIP, preserveDIP: svcInfo.preserveDIP, sessionAffinity: sessionAffinityClientIP, isIPv6: proxier.isIPv6Mode},
						sourceVip,
						lbIngressIP.ip,
						Enum(svcInfo.Protocol()),
						uint16(svcInfo.targetPort),
						uint16(svcInfo.Port()),
						queriedLoadBalancers,
					)
					if err != nil {
						klog.ErrorS(err, "Policy creation failed")
						continue
					}
					lbIngressIP.hnsID = hnsLoadBalancer.hnsID
					klog.V(3).InfoS("Hns LoadBalancer resource created for loadBalancer Ingress resources", "lbIngressIP", lbIngressIP)
				} else {
					klog.V(3).InfoS("Skipped creating Hns LoadBalancer for loadBalancer Ingress resources", "lbIngressIP", lbIngressIP)
				}
				lbIngressIP.hnsID = hnsLoadBalancer.hnsID
				klog.V(3).InfoS("Hns LoadBalancer resource created for loadBalancer Ingress resources", "lbIngressIP", lbIngressIP)

			}
			svcInfo.policyApplied = true
			klog.V(2).InfoS("Policy successfully applied for service", "serviceInfo", svcInfo)
		}
	}

	//metrics.SyncProxyRulesLastTimestamp.SetToCurrentTime()

	// Update service healthchecks.  The endpoints list might include services that are
	// not "OnlyLocal", but the services list will not, and the serviceHealthServer
	// will just drop those endpoints.
	//	if err := proxier.serviceHealthServer.SyncServices(serviceUpdateResult.HCServiceNodePorts); err != nil {
	//		klog.ErrorS(err, "Error syncing healthcheck services")
	//	}
	//	if err := proxier.serviceHealthServer.SyncEndpoints(endpointUpdateResult.HCEndpointsLocalIPSize); err != nil {
	//		klog.ErrorS(err, "Error syncing healthcheck endpoints")
	//	}

	// Finish housekeeping.
	// TODO: these could be made more consistent.
	for _, svcIP := range staleServices.UnsortedList() {
		// TODO : Check if this is required to cleanup stale services here
		klog.V(5).InfoS("Pending delete stale service IP connections", "IP", svcIP)
	}

	// remove stale endpoint refcount entries
	for hnsID, referenceCount := range proxier.endPointsRefCount {
		if *referenceCount <= 0 {
			delete(proxier.endPointsRefCount, hnsID)
		}
	}
}
