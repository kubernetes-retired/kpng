# KPNG EBPF Backend Implementation

## OS pre-requisites

* Linux Kernel > 5.15 (hasn't be tested on earlier versions)
* llvm and clang :
    - Fedora `sudo dnf -y install llvm clang`
    - Ubuntu `sudo apt-get -y install llvm clang`
* libbpf -> v0.8.0 is Automatically downloaded with `make bytecode` target
* [cilium/ebpf requirements](https://github.com/cilium/ebpf#requirements)
* Bpf2go 
    - `go install github.com/cilium/ebpf/cmd/bpf2go@master`

## Intro

NOTE: This KPNG ebpf based backend is currently a POC and is limited in functionality
exclusively to proxying internal ClusterIP based TCP + UDP services.  Functionality 
will be expanded moving forward to include support for the remainder of the defined 
service features.

## Manually download libbpf headers and compile bytecode

This will automatically use `cilium/ebpf` to compile the go program into bytecode
using clang, and build go bindings

`cd /backends/ebpf && make bytecode`

## Start a local kpng ebpf backend kind cluster

Starting a local KIND cluster with the ebpf backend will automatically install 
bpf2go if needed, and recompile the BPF program. 

`./hack/test_e2e.sh -i ipv4 -b ebpf -d`

## Testing Local Changes quickly

1. `docker build -t kpng:test -f Dockerfile .` 
NOTE: If any changes was made to the c source code `go generate` must be manually run 
prior to image building.

2. `kind load docker-image kpng:test --name=kpng-e2e-ipv4-ebpf`

3. `kubectl delete pods -n kube-system -l app=kpng`

## See ebpf program logs

`kubectl logs -f <KPNG_POD_NAME> -n kube-system -c kpng-ebpf-tools cat /tracing/trace_pipe`


## Licensing

The user space components of this example are licensed under the [Apache License, Version 2.0](/LICENSE) as is the
rest of the code defined in KPNG.

The bpf code template (defined in [`cgroup_connect3.c`](/backends/ebpf/bpf/cgroup_connect4.c)) was adapted from
the bpf templates defined in the [Cilium Project](https://github.com/cilium/cilium) and
continues to use the same licenses defined there, i.e the [2-Clause BSD License](/backends/ebpf/bpf/LICENSE.BSD-2-Clause)
and [General Public License, Version 2.0 (only)](/backends/ebpf/bpf/LICENSE.GPL-2.0)
