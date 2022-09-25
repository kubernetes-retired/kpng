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

package userspace

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
	"sigs.k8s.io/kpng/api/localnetv1"
)

const allAvailableInterfaces string = ""

type portal struct {
	ip         string
	port       int
	isExternal bool
}

type serviceInfo struct {
	isAliveAtomic           int32 // Only access this with atomic ops
	portal                  portal
	protocol                localnetv1.Protocol
	socket                  proxySocket
	timeout                 time.Duration
	activeClients           *clientCache
	sessionClientIPAffinity *localnetv1.ClientIPAffinity
}

func (info *serviceInfo) setAlive(b bool) {
	var i int32
	if b {
		i = 1
	}
	atomic.StoreInt32(&info.isAliveAtomic, i)
}

func (info *serviceInfo) isAlive() bool {
	return atomic.LoadInt32(&info.isAliveAtomic) != 0
}

func logTimeout(err error) bool {
	if e, ok := err.(net.Error); ok {
		if e.Timeout() {
			klog.V(3).InfoS("connection to endpoint closed due to inactivity")
			return true
		}
	}
	return false
}

// Provider is a proxy interface enforcing services and endpoints methods
// implementations
type Provider interface {
	EndpointsHandler

	// OnServiceAdd is called whenever creation of new service object
	// is observed.
	OnServiceAdd(service *localnetv1.Service)
	// OnServiceUpdate is called whenever modification of an existing
	// service object is observed.
	OnServiceUpdate(oldService, service *localnetv1.Service)
	// OnServiceDelete is called whenever deletion of an existing service
	// object is observed.
	OnServiceDelete(service *localnetv1.Service)
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

// Proxier is a simple proxy for TCP connections between a localhost:lport
// and services that provide the actual implementations.
type Proxier struct {
	// TODO !!!!
	// EndpointSlice support has not been added for this proxier yet.
	//config.NoopEndpointSliceHandler
	// TODO(imroc): implement node handler for winuserspace proxier.
	//config.NoopNodeHandler

	loadBalancer   LoadBalancer
	mu             sync.Mutex // protects serviceMap
	serviceMap     map[ServicePortPortalName]*serviceInfo
	syncPeriod     time.Duration
	udpIdleTimeout time.Duration
	numProxyLoops  int32 // use atomic ops to access this; mostly for testing
	netsh          Interface
	hostIP         net.IP
}

// Used below.
var localhostIPv4 = netutils.ParseIPSloppy("127.0.0.1")
var localhostIPv6 = netutils.ParseIPSloppy("::1")

var _ Provider = &Proxier{}

var (
	// ErrProxyOnLocalhost is returned by NewProxier if the user requests a proxier on
	// the loopback address. May be checked for by callers of NewProxier to know whether
	// the caller provided invalid input.
	ErrProxyOnLocalhost = fmt.Errorf("cannot proxy on localhost")
)

// NewProxier returns a new Proxier given a LoadBalancer and an address on
// which to listen. It is assumed that there is only a single Proxier active
// on a machine. An error will be returned if the proxier cannot be started
// due to an invalid ListenIP (loopback)
func NewProxier(loadBalancer LoadBalancer, listenIP net.IP, netsh Interface, pr utilnet.PortRange, syncPeriod, udpIdleTimeout time.Duration) (*Proxier, error) {
	if listenIP.Equal(localhostIPv4) || listenIP.Equal(localhostIPv6) {
		return nil, ErrProxyOnLocalhost
	}

	hostIP, err := utilnet.ChooseHostInterface()
	if err != nil {
		return nil, fmt.Errorf("failed to select a host interface: %v", err)
	}

	klog.V(2).InfoS("Setting proxy", "ip", hostIP)
	return createProxier(loadBalancer, listenIP, netsh, hostIP, syncPeriod, udpIdleTimeout)
}

func createProxier(loadBalancer LoadBalancer, listenIP net.IP, netsh Interface, hostIP net.IP, syncPeriod, udpIdleTimeout time.Duration) (*Proxier, error) {
	return &Proxier{
		loadBalancer:   loadBalancer,
		serviceMap:     make(map[ServicePortPortalName]*serviceInfo),
		syncPeriod:     syncPeriod,
		udpIdleTimeout: udpIdleTimeout,
		netsh:          netsh,
		hostIP:         hostIP,
	}, nil
}

// Sync is called to immediately synchronize the proxier state
func (proxier *Proxier) Sync() {
	proxier.cleanupStaleStickySessions()
}

// SyncLoop runs periodic work.  This is expected to run as a goroutine or as the main loop of the app.  It does not return.
func (proxier *Proxier) SyncLoop() {
	t := time.NewTicker(proxier.syncPeriod)
	defer t.Stop()
	for {
		<-t.C
		klog.V(6).InfoS("Periodic sync")
		proxier.Sync()
	}
}

// cleanupStaleStickySessions cleans up any stale sticky session records in the hash map.
func (proxier *Proxier) cleanupStaleStickySessions() {
	proxier.mu.Lock()
	defer proxier.mu.Unlock()
	servicePortNameMap := make(map[ServicePortName]bool)
	for name := range proxier.serviceMap {
		servicePortName := ServicePortName{
			NamespacedName: types.NamespacedName{
				Namespace: name.Namespace,
				Name:      name.Name,
			},
			Port: name.Port,
		}
		if !servicePortNameMap[servicePortName] {
			// ensure cleanup sticky sessions only gets called once per serviceportname
			servicePortNameMap[servicePortName] = true
			proxier.loadBalancer.CleanupStaleStickySessions(servicePortName)
		}
	}
}

// This assumes proxier.mu is not locked.
func (proxier *Proxier) stopProxy(service ServicePortPortalName, info *serviceInfo) error {
	proxier.mu.Lock()
	defer proxier.mu.Unlock()
	return proxier.stopProxyInternal(service, info)
}

// This assumes proxier.mu is locked.
func (proxier *Proxier) stopProxyInternal(service ServicePortPortalName, info *serviceInfo) error {
	delete(proxier.serviceMap, service)
	info.setAlive(false)
	err := info.socket.Close()
	return err
}

func (proxier *Proxier) getServiceInfo(service ServicePortPortalName) (*serviceInfo, bool) {
	proxier.mu.Lock()
	defer proxier.mu.Unlock()
	info, ok := proxier.serviceMap[service]
	return info, ok
}

func (proxier *Proxier) setServiceInfo(service ServicePortPortalName, info *serviceInfo) {
	proxier.mu.Lock()
	defer proxier.mu.Unlock()
	proxier.serviceMap[service] = info
}

// addServicePortPortal starts listening for a new service, returning the serviceInfo.
// The timeout only applies to UDP connections, for now.
func (proxier *Proxier) addServicePortPortal(servicePortPortalName ServicePortPortalName, protocol localnetv1.Protocol, listenIP string, port int, timeout time.Duration) (*serviceInfo, error) {
	var serviceIP net.IP
	if listenIP != allAvailableInterfaces {
		if serviceIP = netutils.ParseIPSloppy(listenIP); serviceIP == nil {
			return nil, fmt.Errorf("could not parse ip '%q'", listenIP)
		}
		// add the IP address.  Node port binds to all interfaces.
		args := proxier.netshIPv4AddressAddArgs(serviceIP)
		if existed, err := proxier.netsh.EnsureIPAddress(args, serviceIP); err != nil {
			return nil, err
		} else if !existed {
			klog.V(3).InfoS("Added ip address to fowarder interface for service", "servicePortPortalName", servicePortPortalName.String(), "addr", net.JoinHostPort(listenIP, strconv.Itoa(port)), "protocol", protocol.String())
		}
	}

	// add the listener, proxy
	sock, err := newProxySocket(protocol, serviceIP, port)
	if err != nil {
		return nil, err
	}
	si := &serviceInfo{
		isAliveAtomic: 1,
		portal: portal{
			ip:         listenIP,
			port:       port,
			isExternal: false,
		},
		protocol:                protocol,
		socket:                  sock,
		timeout:                 timeout,
		activeClients:           newClientCache(),
		sessionClientIPAffinity: nil, // default
	}
	proxier.setServiceInfo(servicePortPortalName, si)

	klog.V(2).InfoS("Proxying for service", "servicePortPortalName", servicePortPortalName.String(), "addr", net.JoinHostPort(listenIP, strconv.Itoa(port)), "protocol", protocol)
	go func(service ServicePortPortalName, proxier *Proxier) {
		defer runtime.HandleCrash()
		atomic.AddInt32(&proxier.numProxyLoops, 1)
		sock.ProxyLoop(service, si, proxier)
		atomic.AddInt32(&proxier.numProxyLoops, -1)
	}(servicePortPortalName, proxier)

	return si, nil
}

func (proxier *Proxier) closeServicePortPortal(servicePortPortalName ServicePortPortalName, info *serviceInfo) error {
	// turn off the proxy
	if err := proxier.stopProxy(servicePortPortalName, info); err != nil {
		return err
	}

	// close the PortalProxy by deleting the service IP address
	if info.portal.ip != allAvailableInterfaces {
		serviceIP := netutils.ParseIPSloppy(info.portal.ip)
		args := proxier.netshIPv4AddressDeleteArgs(serviceIP)
		if err := proxier.netsh.DeleteIPAddress(args); err != nil {
			return err
		}
	}
	return nil
}

// getListenIPPortMap returns a slice of all listen IPs for a service.
func getListenIPPortMap(service *localnetv1.Service, listenPort, nodePort int) map[string]int {
	listenIPPortMap := make(map[string]int)

	for _, ip := range service.IPs.GetClusterIPs().All() {
		listenIPPortMap[ip] = listenPort
	}

	for _, ip := range service.IPs.GetExternalIPs().All() {
		listenIPPortMap[ip] = listenPort
	}

	for _, ip := range service.IPs.GetLoadBalancerIPs().All() {
		listenIPPortMap[ip] = listenPort
	}

	if nodePort != 0 {
		listenIPPortMap[allAvailableInterfaces] = nodePort
	}

	return listenIPPortMap
}

func (proxier *Proxier) mergeService(service *localnetv1.Service) map[ServicePortPortalName]bool {
	if service == nil {
		return nil
	}
	svcName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	if service.IPs.ClusterIPs == nil {
		klog.V(3).InfoS("Skipping service due to clusterIP", "svcName", svcName, "ip", service.IPs.GetClusterIPs())
		return nil
	}
	existingPortPortals := make(map[ServicePortPortalName]bool)

	for i := range service.Ports {
		servicePort := &service.Ports[i]
		// create a slice of all the source IPs to use for service port portals

		listenIPPortMap := getListenIPPortMap(service, int((*servicePort).GetPort()), int((*servicePort).GetNodePort()))
		protocol := (*servicePort).Protocol

		for listenIP, listenPort := range listenIPPortMap {
			servicePortPortalName := ServicePortPortalName{
				NamespacedName: svcName,
				Port:           (*servicePort).GetName(),
				PortalIPName:   listenIP,
			}
			existingPortPortals[servicePortPortalName] = true
			info, exists := proxier.getServiceInfo(servicePortPortalName)
			if exists && sameConfig(info, service, protocol, listenPort) {
				// Nothing changed.
				continue
			}
			if exists {
				klog.V(4).InfoS("Something changed for service: stopping it", "servicePortPortalName", servicePortPortalName.String())
				if err := proxier.closeServicePortPortal(servicePortPortalName, info); err != nil {
					klog.ErrorS(err, "Failed to close service port portal", "servicePortPortalName", servicePortPortalName.String())
				}
			}
			klog.V(1).InfoS("Adding new service", "servicePortPortalName", servicePortPortalName.String(), "addr", net.JoinHostPort(listenIP, strconv.Itoa(listenPort)), "protocol", protocol)
			info, err := proxier.addServicePortPortal(servicePortPortalName, protocol, listenIP, listenPort, proxier.udpIdleTimeout)
			if err != nil {
				klog.ErrorS(err, "Failed to start proxy", "servicePortPortalName", servicePortPortalName.String())
				continue
			}
			if service.GetClientIP() != nil {
				info.sessionClientIPAffinity = service.GetClientIP()
			}
			klog.V(10).InfoS("record serviceInfo", "info", info)
		}
		if len(listenIPPortMap) > 0 {
			// only one loadbalancer per service port portal
			servicePortName := ServicePortName{
				NamespacedName: types.NamespacedName{
					Namespace: service.Namespace,
					Name:      service.Name,
				},
				Port: (*servicePort).GetName(),
			}
			timeoutSeconds := 0
			if service.SessionAffinity != nil {
				timeoutSeconds = int(service.GetClientIP().TimeoutSeconds)
			}
			proxier.loadBalancer.NewService(servicePortName, service.GetClientIP(), timeoutSeconds)
		}
	}

	return existingPortPortals
}

func (proxier *Proxier) unmergeService(service *localnetv1.Service, existingPortPortals map[ServicePortPortalName]bool) {
	if service == nil {
		return
	}
	svcName := types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
	if service.IPs.ClusterIPs == nil {
		klog.V(3).InfoS("Skipping service due to clusterIP", "svcName", svcName, "ip", service.IPs.GetClusterIPs())
		return
	}

	servicePortNameMap := make(map[ServicePortName]bool)
	for name := range existingPortPortals {
		servicePortName := ServicePortName{
			NamespacedName: types.NamespacedName{
				Namespace: name.Namespace,
				Name:      name.Name,
			},
			Port: name.Port,
		}
		servicePortNameMap[servicePortName] = true
	}

	for i := range service.Ports {
		servicePort := &service.Ports[i]
		serviceName := ServicePortName{NamespacedName: svcName, Port: (*servicePort).GetName()}
		// create a slice of all the source IPs to use for service port portals
		listenIPPortMap := getListenIPPortMap(service, int((*servicePort).GetPort()), int((*servicePort).GetNodePort()))

		for listenIP := range listenIPPortMap {
			servicePortPortalName := ServicePortPortalName{
				NamespacedName: svcName,
				Port:           (*servicePort).GetName(),
				PortalIPName:   listenIP,
			}
			if existingPortPortals[servicePortPortalName] {
				continue
			}

			klog.V(1).InfoS("Stopping service", "servicePortPortalName", servicePortPortalName.String())
			info, exists := proxier.getServiceInfo(servicePortPortalName)
			if !exists {
				klog.ErrorS(nil, "Service is being removed but doesn't exist", "servicePortPortalName", servicePortPortalName.String())
				continue
			}

			if err := proxier.closeServicePortPortal(servicePortPortalName, info); err != nil {
				klog.ErrorS(err, "Failed to close service port portal", "servicePortPortalName", servicePortPortalName)
			}
		}

		// Only delete load balancer if all listen ips per name/port show inactive.
		if !servicePortNameMap[serviceName] {
			proxier.loadBalancer.DeleteService(serviceName)
		}
	}
}

// OnServiceAdd is called whenever creation of new service object
// is observed.
func (proxier *Proxier) OnServiceAdd(service *localnetv1.Service) {
	_ = proxier.mergeService(service)
}

// OnServiceUpdate is called whenever modification of an existing
// service object is observed.
func (proxier *Proxier) OnServiceUpdate(oldService, service *localnetv1.Service) {
	existingPortPortals := proxier.mergeService(service)
	proxier.unmergeService(oldService, existingPortPortals)
}

// OnServiceDelete is called whenever deletion of an existing service
// object is observed.
func (proxier *Proxier) OnServiceDelete(service *localnetv1.Service) {
	proxier.unmergeService(service, map[ServicePortPortalName]bool{})
}

// OnServiceSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnServiceSynced() {
}

// OnEndpointsAdd is called whenever creation of new endpoints object
// is observed.
func (proxier *Proxier) OnEndpointsAdd(ep *localnetv1.Endpoint, svc *localnetv1.Service) {
	proxier.loadBalancer.OnEndpointsAdd(ep, svc)
}

// OnEndpointsUpdate is called whenever modification of an existing
// endpoints object is observed.
func (proxier *Proxier) OnEndpointsUpdate(oldEndpoints, endpoints *localnetv1.Endpoint) {
	//proxier.loadBalancer.OnEndpointsUpdate(oldEndpoints, endpoints)
}

// OnEndpointsDelete is called whenever deletion of an existing endpoints
// object is observed. Service object
func (proxier *Proxier) OnEndpointsDelete(ep *localnetv1.Endpoint, svc *localnetv1.Service) {
	proxier.loadBalancer.OnEndpointsDelete(ep, svc)
}

// OnEndpointsSynced is called once all the initial event handlers were
// called and the state is fully propagated to local cache.
func (proxier *Proxier) OnEndpointsSynced() {
	proxier.loadBalancer.OnEndpointsSynced()
}

func isTooManyFDsError(err error) bool {
	return strings.Contains(err.Error(), "too many open files")
}

func isClosedError(err error) bool {
	// A brief discussion about handling closed error here:
	// https://code.google.com/p/go/issues/detail?id=4373#c14
	// TODO: maybe create a stoppable TCP listener that returns a StoppedError
	return strings.HasSuffix(err.Error(), "use of closed network connection")
}

func sameConfig(info *serviceInfo, service *localnetv1.Service, protocol localnetv1.Protocol, listenPort int) bool {
	return info.protocol == protocol && info.portal.port == listenPort && info.sessionClientIPAffinity != service.GetClientIP()
}

func (proxier *Proxier) netshIPv4AddressAddArgs(destIP net.IP) []string {
	intName := proxier.netsh.GetInterfaceToAddIP()
	args := []string{
		"interface", "ipv4", "add", "address",
		"name=" + intName,
		"address=" + destIP.String(),
	}

	return args
}

func (proxier *Proxier) netshIPv4AddressDeleteArgs(destIP net.IP) []string {
	intName := proxier.netsh.GetInterfaceToAddIP()
	args := []string{
		"interface", "ipv4", "delete", "address",
		"name=" + intName,
		"address=" + destIP.String(),
	}

	return args
}
