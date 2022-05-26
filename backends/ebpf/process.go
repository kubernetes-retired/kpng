//go:build linux
// +build linux

package ebpf

import (
	"bufio"
	"errors"
	"log"
	"os"
	"path"
	"strings"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

const bpfFSPath = "/sys/fs/bpf"

//go:generate bpf2go -cc clang -cflags "-O2 -g -Wall -Werror" bpf cgroup_connect4.c -- -I./headers
func ebpfSetup() ebpfController {
	var err error
	// Name of the kernel function we're tracing
	fn := "count_sock4_connect"

	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		klog.Fatal(err)
	}

	pinPath := path.Join(bpfFSPath, fn)
	if err := os.MkdirAll(pinPath, os.ModePerm); err != nil {
		klog.Fatalf("failed to create bpf fs subpath: %+v", err)
	}

	klog.Infof("Pin Path is %s", pinPath)

	// Load pre-compiled programs and maps into the kernel.
	objs := bpfObjects{}
	if err := loadBpfObjects(&objs, &cebpf.CollectionOptions{
		Maps: cebpf.MapOptions{
			// Pin the map to the BPF filesystem and configure the
			// library to automatically re-write it in the BPF
			// program so it can be re-used if it already exists or
			// create it if not
			PinPath: pinPath,
		},
	}); err != nil {
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
