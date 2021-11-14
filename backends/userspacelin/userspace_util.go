package userspacelin

import (
	"fmt"
	"net"
	"strconv"

	v1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/backends/iptables"
)

// ShouldSkipService checks if a given service should skip proxying
func ShouldSkipService(service *localnetv1.Service) bool {
	// if ClusterIP is "None" or empty, skip proxying
	if !iptables.IsServiceIPSet(service) {
		klog.V(3).Infof("Skipping service %s in namespace %s due to clusterIP = %q", service.Name, service.Namespace, service.IPs.ClusterIPs.V4[0])
		return true
	}
	// Even if ClusterIP is set, ServiceTypeExternalName services don't get proxied
	if service.Type == string(v1.ServiceTypeExternalName) {
		klog.V(3).Infof("Skipping service %s in namespace %s due to Type=ExternalName", service.Name, service.Namespace)
		return true
	}
	return false
}

// isValidEndpoint checks that the given host / port pair are valid endpoint
func isValidEndpoint(host string, port int) bool {
	return host != "" && port > 0
}

// ToCIDR returns a host address of the form <ip-address>/32 for
// IPv4 and <ip-address>/128 for IPv6
func ToCIDR(ip net.IP) string {
	len := 32
	if ip.To4() == nil {
		len = 128
	}
	return fmt.Sprintf("%s/%d", ip.String(), len)
}

// BuildPortsToEndpointsMap builds a map of portname -> all ip:ports for that
// portname. Explode Endpoints.Subsets[*] into this structure.
func BuildPortsToEndpointsMap(service []*iptables.ServicePortName, endpoints *localnetv1.Endpoint) map[string][]string {
	portsToEndpoints := map[string][]string{}
	ipSet := endpoints.GetIPs()
	for _, i := range ipSet.V4 {
		for _, svc := range service {
			intt, _ := strconv.Atoi(svc.Port)
			if isValidEndpoint(i, intt) {
				//append 10.1.2.3:8080 to "a"
				portsToEndpoints[svc.PortName] = append(portsToEndpoints[svc.PortName], net.JoinHostPort(i, svc.Port))
			}
		}
	}
	// {
	// "a": {10.1.1.1:80, 10.2.2.2:80}
	// "b" : {10.1.1.1:443, 10.2.2.2:443}
	// }
	return portsToEndpoints
}

// GetLocalAddrs returns a list of all network addresses on the local system
func GetLocalAddrs() ([]net.IP, error) {
	var localAddrs []net.IP
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			return nil, err
		}
		localAddrs = append(localAddrs, ip)
	}
	return localAddrs, nil
}

// GetLocalAddrSet return a local IPSet.
// If failed to get local addr, will assume no local ips.
func GetLocalAddrSet() utilnet.IPSet {
	localAddrs, err := GetLocalAddrs()
	if err != nil {
		klog.ErrorS(err, "Failed to get local addresses assuming no local IPs")
	} else if len(localAddrs) == 0 {
		klog.InfoS("No local addresses were found")
	}
	localAddrSet := utilnet.IPSet{}
	localAddrSet.Insert(localAddrs...)
	return localAddrSet
}
