# KPNG EBPF Backend Implementation 

## Compile ebpf program 

This will automatically use cillium/ebpf to compile the go program into bytecode
using clang, and build go bindings

`go generate ./...`

## Run ebpf program 

go run -exec sudo ./ebpfProxy

mount bpffs /sys/fs/bpf -t bpf