# KPNG EBPF Backend Implementation 

## Compile ebpf program 

This will automatically use cillium/ebpf to compile the go program into bytecode
using clang, and build go bindings

`go generate`

## Licensing 

The user space components of this example are licensed under the [Apache License, Version 2.0](/LICENSE) as is the
rest of the code defined in KPNG.

The bpf code template (defined in [`cgroup_connect3.c`](/backends/ebpf/bpf/cgroup_connect4.c)) was adapted from
the bpf templates defined in the [Cilium Project](https://github.com/cilium/cilium) and
continues to use the same licenses defined there, i.e the [2-Clause BSD License](/backends/ebpf/bpf/LICENSE.BSD-2-Clause)
and [General Public License, Version 2.0 (only)](/backends/ebpf/bpf/LICENSE.GPL-2.0)
