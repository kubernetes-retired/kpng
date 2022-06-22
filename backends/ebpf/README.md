# KPNG EBPF Backend Implementation

## Intro

NOTE: This KPNG ebpf based backend is currently a POC and is limited in functionality
exclusively to proxying internal ClusterIP based TCP + UDP services.  Functionality 
will be expanded moving forward to include support for the remainder of the defined 
service features.

## Compile ebpf program

This will automatically use cillium/ebpf to compile the go program into bytecode
using clang, and build go bindings

`go generate`

## Start a local kpng ebpf backend kind cluster

`./hack/test_e2e.sh -i ipv4 -b ebpf -d`

## Testing Local Changes quickly

1. `docker build -t kpng:test -f Dockerfile .` 
NOTE: If any changes was made to the c source code `go generate` much be run 
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
