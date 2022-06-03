package ebpf

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"

	"github.com/cilium/ebpf"
	cebpflink "github.com/cilium/ebpf/link"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client"
)

type svcEndpointMapping struct {
	Svc *BaseServiceInfo

	Endpoint []*localnetv1.Endpoint
}

type ebpfController struct {
	mu         sync.Mutex        // protects the following fields
	nodeLabels map[string]string //TODO: looks like can be removed as kpng controller shoujld do the work

	// Keeps track of ebpf objects in memory.
	objs bpfObjects

	// Program Link,
	bpfLink cebpflink.Link

	ipFamily v1.IPFamily

	// Caches of what service info our ebpf MAPs should contain
	svcMap map[ServicePortName]svcEndpointMapping
}

func NewEBPFController(objs bpfObjects, bpfProgLink cebpflink.Link, ipFamily v1.IPFamily) ebpfController {
	return ebpfController{
		objs:     objs,
		bpfLink:  bpfProgLink,
		ipFamily: ipFamily,
		svcMap:   make(map[ServicePortName]svcEndpointMapping),
	}
}

func (ebc *ebpfController) Cleanup() {
	klog.Info("Cleaning Up EBPF resources")
	ebc.bpfLink.Close()
	ebc.objs.Close()
}

func (ebc *ebpfController) Callback(ch <-chan *client.ServiceEndpoints) {
	// Populate internal cache based on incoming information
	for serviceEndpoints := range ch {
		klog.Infof("Iterating fullstate channel, got: %+v", serviceEndpoints)

		if serviceEndpoints.Service.Type != "ClusterIP" {
			klog.Warning("Ebpf Proxy not yet implemented for svc types other than clusterIP")
			continue
		}

		svcKey := types.NamespacedName{Name: serviceEndpoints.Service.Name, Namespace: serviceEndpoints.Service.Namespace}

		keysNeedingSync := []ServicePortName{}

		for i := range serviceEndpoints.Service.Ports {
			servicePort := serviceEndpoints.Service.Ports[i]
			svcPortName := ServicePortName{NamespacedName: svcKey, Port: servicePort.Name, Protocol: servicePort.Protocol}
			baseSvcInfo := ebc.newBaseServiceInfo(servicePort, serviceEndpoints.Service)
			svcEndptRelation := svcEndpointMapping{Svc: baseSvcInfo, Endpoint: serviceEndpoints.Endpoints}

			existing, ok := ebc.svcMap[svcPortName]

			// Always update cache regardless of if sync is needed
			// Eventually we'll spawn multiple go routines to handle this, and then
			// we'll need the data lock
			ebc.mu.Lock()
			ebc.svcMap[svcPortName] = svcEndptRelation
			ebc.mu.Unlock()

			// If svc did not exist, sync
			if !ok {
				keysNeedingSync = append(keysNeedingSync, svcPortName)
				continue
			}

			// If svc changed, sync
			if existing.Svc != svcEndptRelation.Svc {
				keysNeedingSync = append(keysNeedingSync, svcPortName)
			}

			// if # svc endpoints changed sync
			if len(existing.Endpoint) != len(svcEndptRelation.Endpoint) {
				keysNeedingSync = append(keysNeedingSync, svcPortName)
				continue
			}

			// if svc endpoints changed sync
			for i, _ := range existing.Endpoint {
				if existing.Endpoint[i] != svcEndptRelation.Endpoint[i] {
					keysNeedingSync = append(keysNeedingSync, svcPortName)
					break
				}
			}
		}

		// Reconcile what we have in ebc.svcInfo to internal cache and ebpf maps
		if len(keysNeedingSync) != 0 {
			ebc.Sync(keysNeedingSync)
		}

	}
}

// Sync will take the new internally cached state and apply it to the bpf maps
// fully syncing the maps on every iteration.
func (ebc *ebpfController) Sync(keys []ServicePortName) {
	for _, key := range keys {
		svcInfo := ebc.svcMap[key]

		svcKeys, svcValues, backendKeys, backendValues := makeEbpfMaps(svcInfo)

		if _, err := ebc.objs.V4SvcMap.BatchUpdate(svcKeys, svcValues, &ebpf.BatchOptions{}); err != nil {
			klog.Fatalf("Failed Loading service entries: %v", err)
			ebc.Cleanup()
		}

		if _, err := ebc.objs.V4BackendMap.BatchUpdate(backendKeys, backendValues, &ebpf.BatchOptions{}); err != nil {
			klog.Fatalf("Failed Loading service backend entries: %v", err)
			ebc.Cleanup()
		}
	}
}

func makeEbpfMaps(svcMapping svcEndpointMapping) (svcKeys []Service4Key, svcValues []Service4Value,
	backendKeys []Backend4Key, backendValues []Backend4Value) {
	// Make sure what we store here is in network endian
	var svcAddress [4]byte
	var svcPort [2]byte
	var targetPort [2]byte
	var backendAddress [4]byte
	var ID uint32
	var err error
	addresses := []string{}

	copy(svcAddress[:], svcMapping.Svc.clusterIP.To4())

	klog.Infof("Got SvcMapping %+v", svcMapping)

	// Hack for service Port name
	binary.BigEndian.PutUint16(targetPort[:], uint16(svcMapping.Svc.targetPort))
	binary.BigEndian.PutUint16(svcPort[:], uint16(svcMapping.Svc.port))

	for _, endpoint := range svcMapping.Endpoint {
		addresses = append(addresses, endpoint.IPs.V4...)
	}

	// Make root (backendID 0, count != 0) key/value for service
	svcKeys = append(svcKeys, Service4Key{
		Address:     svcAddress,
		Port:        svcPort,
		BackendSlot: 0,
	})

	svcValues = append(svcValues, Service4Value{Count: uint16(len(addresses))})

	// Make rest of svc and backend entries for service
	for i, address := range addresses {
		i := i
		copy(backendAddress[:], net.ParseIP(address).To4())

		svcKeys = append(svcKeys, Service4Key{
			Address:     svcAddress,
			Port:        svcPort,
			BackendSlot: uint16(i + 1),
		})

		// Make backendID the int value of the string version of the address + int protocol value
		err = binary.Read(bytes.NewBuffer(net.ParseIP(address).To4()), binary.BigEndian, &ID)
		if err != nil {
			klog.Errorf("Failed to convert endpoint address: %s to Int32, err : %v",
				address, err)
		}
		// Increment by port to have unique backend value for each svcPort
		ID = ID + uint32(svcMapping.Svc.port)

		svcValues = append(svcValues, Service4Value{Count: 0,
			BackendID: ID,
		})

		backendKeys = append(backendKeys, Backend4Key{
			ID: uint32(ID),
		})

		backendValues = append(backendValues, Backend4Value{
			Address: backendAddress,
			Port:    targetPort,
		})
	}
	klog.Infof("Writing svcKeys %+v \nsvcValues %+v \nbackendKeys %+v \n backendValues %+v",
		svcKeys, svcValues, backendKeys, backendValues)

	return svcKeys, svcValues, backendKeys, backendValues
}

// // mapToEbpfProto takes a proto as defined by KPNG and maps it to those defined by
// // linux in https://github.com/torvalds/linux/blob/master/include/uapi/linux/in.h#L27
// func mapToEbpfProto(kpngProto int) U8proto {
// 	switch kpngProto {
// 	// TCP
// 	case 1:
// 		return 6
// 	// UDP
// 	case 2:
// 		return 17
// 	// SCTP
// 	case 3:
// 		return 132
// 	default:
// 		return 0
// 	}
// }

// Types used to interact with ebpf maps

// ServiceEndpoint is used to identify a service and one of its endpoint pair.
type ServiceEndpoint struct {
	Endpoint        string
	ServicePortName ServicePortName
}

// Service4Value must match 'struct lb4_service_v2' in "bpf/lib/common.h".
type Service4Value struct {
	BackendID uint32
	Count     uint16
	RevNat    uint16
	Flags     uint8
	Flags2    uint8
	Pad       pad2uint8
}

// Service4Key must match 'struct lb4_key' in "bpf/lib/common.h".
type Service4Key struct {
	Address     IPv4
	Port        Port
	BackendSlot uint16
}

// Backend4Value must match 'struct lb4_backend' in "bpf/lib/common.h".
type Backend4Value struct {
	Address IPv4
	Port    Port
	Flags   uint8
	Pad     uint8
}

type pad2uint8 [2]uint8

type IPv4 [4]byte
type Port [2]byte

type Backend4Key struct {
	ID uint32
}
