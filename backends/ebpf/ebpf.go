package ebpf

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"

	"sigs.k8s.io/kpng/client"
)

//go:generate bpf2go -cc clang -cflags "-O2 -g -Wall -Werror" bpf ./bpf/cgroup_connect4.c -- -I./bpf/headers
func ebpfSetup() ebpfController {
	var err error

	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		klog.Fatal(err)
	}

	// Load pre-compiled programs and maps into the kernel.
	objs := bpfObjects{}
	if err := loadBpfObjects(&objs, &cebpf.CollectionOptions{}); err != nil {
		log.Fatalf("loading objects: %v", err)
	}

	info, err := objs.bpfMaps.V4SvcMap.Info()
	if err != nil {
		klog.Fatalf("Cannot get map info: %v", err)
	}
	klog.Infof("Svc Map Info: %+v with FD %s", info, objs.bpfMaps.V4SvcMap.String())

	info, err = objs.bpfMaps.V4BackendMap.Info()
	if err != nil {
		klog.Fatalf("Cannot get map info: %v", err)
	}
	klog.Infof("Backend Map Info: %+v", info)

	// Get the first-mounted cgroupv2 path.
	cgroupPath, err := detectRootCgroupPath()
	if err != nil {
		log.Fatal(err)
	}

	klog.Infof("Cgroup Path is %s", cgroupPath)

	// Link the proxy program to the default cgroup.
	l, err := link.AttachCgroup(link.CgroupOptions{
		Path:    cgroupPath,
		Attach:  cebpf.AttachCGroupInet4Connect,
		Program: objs.Sock4Connect,
	})
	if err != nil {
		klog.Fatal(err)
	}

	klog.Infof("Proxying packets in kernel...")

	return NewEBPFController(objs, l, v1.IPv4Protocol)
}

// detectCgroupPath returns the first-found mount point of type cgroup2
// and stores it in the cgroupPath global variable.
func detectRootCgroupPath() (string, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// example fields: cgroup2 /sys/fs/cgroup/unified cgroup2 rw,nosuid,nodev,noexec,relatime 0 0
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) >= 3 && fields[2] == "cgroup2" {
			return fields[1], nil
		}
	}

	return "", errors.New("cgroup2 not mounted")
}

func (ebc *ebpfController) Cleanup() {
	klog.Info("Cleaning Up EBPF resources")
	ebc.bpfLink.Close()
	ebc.objs.Close()
}

func (ebc *ebpfController) Callback(ch <-chan *client.ServiceEndpoints) {
	// Used to determine what services are stale in bpf
	currentSvcKeys := sets.NewString()
	// Services that need to be updated
	keysNeedingSync := []string{}

	// Populate internal cache based on incoming fullstate information
	for serviceEndpoints := range ch {
		klog.Infof("Iterating fullstate channel, got: %+v", serviceEndpoints)

		klog.Infof("Hi Its Andrew")
		if serviceEndpoints.Service.Type != "ClusterIP" {
			klog.Warning("Ebpf Proxy not yet implemented for svc types other than clusterIP")
			continue
		}

		svcUniqueName := types.NamespacedName{Name: serviceEndpoints.Service.Name, Namespace: serviceEndpoints.Service.Namespace}

		for i := range serviceEndpoints.Service.Ports {
			servicePort := serviceEndpoints.Service.Ports[i]
			svcKey := fmt.Sprintf("%s%d%s", svcUniqueName, servicePort.Port, servicePort.Protocol)
			baseSvcInfo := ebc.newBaseServiceInfo(servicePort, serviceEndpoints.Service)
			svcEndptRelation := svcEndpointMapping{Svc: baseSvcInfo, Endpoint: serviceEndpoints.Endpoints}

			currentSvcKeys.Insert(svcKey)
			existing, ok := ebc.svcMap[svcKey]

			// Always update cache regardless of if sync is needed
			// Eventually we'll spawn multiple go routines to handle this, and then
			// we'll need the data lock
			ebc.mu.Lock()
			ebc.svcMap[svcKey] = svcEndptRelation
			ebc.svcMapKeys.Insert(svcKey)
			ebc.mu.Unlock()

			// If svc did not exist, sync
			if !ok {
				keysNeedingSync = append(keysNeedingSync, svcKey)
				continue
			}

			// If svc changed, sync
			if existing.Svc != svcEndptRelation.Svc {
				keysNeedingSync = append(keysNeedingSync, svcKey)
			}

			// if # svc endpoints changed sync
			if len(existing.Endpoint) != len(svcEndptRelation.Endpoint) {
				keysNeedingSync = append(keysNeedingSync, svcKey)
				continue
			}

			// if svc endpoints changed sync
			for i, _ := range existing.Endpoint {
				if existing.Endpoint[i] != svcEndptRelation.Endpoint[i] {
					keysNeedingSync = append(keysNeedingSync, svcKey)
					break
				}
			}
		}

	}

	// Reconcile what we have in ebc.svcInfo to internal cache and ebpf maps
	if len(keysNeedingSync) != 0 || !ebc.svcMapKeys.Equal(currentSvcKeys) {
		ebc.Sync(keysNeedingSync, ebc.svcMapKeys.Difference(currentSvcKeys).List())
		// Update cache of svc keys
		ebc.svcMapKeys = currentSvcKeys
	}
}

// Sync will take the new internally cached state and apply it to the bpf maps
// fully syncing the maps on every iteration.
func (ebc *ebpfController) Sync(syncKeys, deleteKeys []string) {

	for _, key := range deleteKeys {
		svcInfo := ebc.svcMap[key]

		svcKeys, _, backendKeys, _ := makeEbpfMaps(svcInfo)

		if _, err := ebc.objs.V4SvcMap.BatchDelete(svcKeys, &cebpf.BatchOptions{}); err != nil {
			// Look at not crashing here.
			klog.Fatalf("Failed Deleting service entries: %v", err)
			ebc.Cleanup()
		}

		if _, err := ebc.objs.V4BackendMap.BatchDelete(backendKeys, &cebpf.BatchOptions{}); err != nil {
			klog.Fatalf("Failed Deleting service backend entries: %v", err)
			ebc.Cleanup()
		}
		// Remove service entry from cache
		delete(ebc.svcMap, key)
	}

	for _, key := range syncKeys {
		svcInfo := ebc.svcMap[key]

		svcKeys, svcValues, backendKeys, backendValues := makeEbpfMaps(svcInfo)

		if _, err := ebc.objs.V4SvcMap.BatchUpdate(svcKeys, svcValues, &cebpf.BatchOptions{}); err != nil {
			klog.Fatalf("Failed Loading service entries: %v", err)
			ebc.Cleanup()
		}

		if _, err := ebc.objs.V4BackendMap.BatchUpdate(backendKeys, backendValues, &cebpf.BatchOptions{}); err != nil {
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
	klog.V(5).Infof("Writing svcKeys %+v \nsvcValues %+v \nbackendKeys %+v \nbackendValues %+v",
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
