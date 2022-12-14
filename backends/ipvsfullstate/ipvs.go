package ipvsfullsate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cespare/xxhash"
	"github.com/vishvananda/netlink"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/exec"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/util"
	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/lightdiffstore"
	"sync"
)

type IpvsController struct {
	mu sync.Mutex

	ipFamily v1.IPFamily
	svcSet   sets.String
	svcStore *lightdiffstore.DiffStore
	epStore  *lightdiffstore.DiffStore

	iptables util.IPTableInterface
	ipset    util.Interface
	exec     exec.Interface

	proxier *proxier
}

func NewIPVSController(dummy netlink.Link) IpvsController {
	execer := exec.New()
	ipsetInterface := util.New(execer)
	iptInterface := util.NewIPTableInterface(execer, util.Protocol(v1.IPv4Protocol))

	masqueradeBit := 14
	masqueradeValue := 1 << uint(masqueradeBit)
	masqueradeMark := fmt.Sprintf("%#08x", masqueradeValue)

	ipv4Proxier := NewProxier(
		v1.IPv4Protocol,
		dummy,
		ipsetInterface,
		iptInterface,
		interfaceAddresses(),
		IPVSSchedulingMethod,
		masqueradeMark,
		true,
		IPVSWeight,
	)

	ipv4Proxier.initializeIPSets()

	return IpvsController{
		svcStore: lightdiffstore.New(),
		epStore:  lightdiffstore.New(),
		ipFamily: v1.IPv4Protocol,
		proxier:  ipv4Proxier,
	}
}

func (c *IpvsController) Callback(ch <-chan *client.ServiceEndpoints) {

	c.svcStore.Reset(lightdiffstore.ItemDeleted)
	c.epStore.Reset(lightdiffstore.ItemDeleted)

	for serviceEndpoints := range ch {

		if serviceEndpoints.Service.Type != ClusterIPService {
			klog.Warning("IPVS Proxy not yet implemented for svc types other than clusterIP")
			continue
		}

		service := serviceEndpoints.Service
		endpoints := serviceEndpoints.Endpoints

		for _, port := range service.Ports {

			svcKey := fmt.Sprintf("[%s|%d|%s]", service.NamespacedName(), port.Port, port.Protocol)
			clusterIP := GetClusterIPByFamily(v1.IPv4Protocol, service)
			servicePortInfo := NewServicePortInfo(service, port, clusterIP, ClusterIPService, IPVSSchedulingMethod, IPVSWeight)

			// JSON encoding of our services + EP information
			servicePortInfoBytes := new(bytes.Buffer)
			_ = json.NewEncoder(servicePortInfoBytes).Encode(*servicePortInfo)
			c.svcStore.Set([]byte(svcKey), xxhash.Sum64(servicePortInfoBytes.Bytes()), *servicePortInfo)

			for _, endpoint := range endpoints {
				for _, endpointIP := range endpoint.GetIPs().V4 {
					epKey := fmt.Sprintf("[%d|%s|%s]", port.Port, port.Protocol, endpointIP)
					servicePortEndpointInfo := ServicePortEndpointInfo{ServicePortInfo: servicePortInfo, EndpointsInfo: &EndPointInfo{
						endPointIP:      endpointIP,
						isLocalEndPoint: endpoint.GetLocal(),
						portMap:         endpoint.PortNameMappings(service.Ports),
					}}

					// JSON encoding of our services + EP information
					servicePortEndpointInfoBytes := new(bytes.Buffer)
					_ = json.NewEncoder(servicePortEndpointInfoBytes).Encode(servicePortEndpointInfo)
					c.epStore.Set([]byte(epKey), xxhash.Sum64(servicePortEndpointInfoBytes.Bytes()), servicePortEndpointInfo)
				}
			}
		}

		klog.V(3).Infof("received service %s with %d endpoints", serviceEndpoints.Service.NamespacedName(), len(serviceEndpoints.Endpoints))
	}

	if len(c.svcStore.Updated()) != 0 || len(c.svcStore.Deleted()) != 0 {
		c.SyncService()
	}

	if len(c.epStore.Updated()) != 0 || len(c.epStore.Deleted()) != 0 {
		c.SyncEndpoints()
	}
}

func (c *IpvsController) SyncService() {
	// Deleted
	for _, KV := range c.svcStore.Deleted() {
		servicePortInfo := KV.Value.(ServicePortInfo)

		klog.Infof("Deleting ServicePort: %s", string(KV.Key))
		c.deleteServicePortInfo(&servicePortInfo)

		//c.svcStore.Delete(KV.Key)
		//c.svcStore.Reset(lightdiffstore.ItemDeleted)
	}

	// Updated
	for _, KV := range c.svcStore.Updated() {
		servicePortInfo := KV.Value.(ServicePortInfo)

		klog.Infof("Updating ServicePort: %s", string(KV.Key))
		c.updateServicePortInfo(&servicePortInfo)
	}

}

func (c *IpvsController) SyncEndpoints() {
	// Deleted
	for _, KV := range c.epStore.Deleted() {
		servicePortEndpointInfo := KV.Value.(ServicePortEndpointInfo)

		klog.Infof("Deleting ServicePort: %s", string(KV.Key))
		c.deleteEndpointInfo(servicePortEndpointInfo.ServicePortInfo, servicePortEndpointInfo.EndpointsInfo)

		//c.epStore.Delete(KV.Key)
		//c.epStore.Reset(lightdiffstore.ItemDeleted)
	}

	// Updated
	for _, KV := range c.epStore.Updated() {
		servicePortEndpointInfo := KV.Value.(ServicePortEndpointInfo)

		klog.Infof("Updating ServicePort: %s", string(KV.Key))
		c.updateEndpointInfo(servicePortEndpointInfo.ServicePortInfo, servicePortEndpointInfo.EndpointsInfo)
	}
}
