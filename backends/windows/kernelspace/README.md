# Windows kernel space proxy

This is ported from upstream k8s... it uses the service Change tracker, but
eventually will be replaced with https://github.com/kubernetes-sigs/kpng/issues/215

## Testing

### phase 0: windows basics

- get a windows cluster (sig-windows-dev-tools)

### phase 1: basic compilation 
- compile this go module as a windows executable `GOOS=windows go build -trimpath -o ../dist`
- cp this file into sig-windows-dev-tools/ and mount vagrant as shared into windows node
- `vagrant ssh` into windows node, and run this file

Ref: [make: add support for build multi-arch bin](https://github.com/kubernetes-sigs/kpng/pull/219)

### phase 2a: test if kpng server can run as windows process

probably easier to start - mimic existing kube proxy by running kpng brain and kpng win backend in same windows
host use the pure in memory : https://jayunit100.blogspot.com/2021/11/quick-note-on-running-kpng-in-memory.html


```
kpng.exe --kubeconfig=C:/doug/blah.yaml --to-local=to-windows
```
### phase 2b: ditch 2a, and try to run kpng server on LINUX node, and have it talk over IP to WINDOWS node

remote GRPC as opposed to local file socket this will prevent the windows node
from needing to run a kpng server

### phase 3: port the exe from phase 1, into pod

using host-process containers, windows can run kpng proxy probably as a pod entirely
that will be something we can do once we finish the initial merge of this backend.


# homework
- [Episode 144 : Exploring The State of K8s on Windows](https://github.com/vmware-tanzu/tgik/tree/master/episodes/144)
- [sig-windows-dev-tools](https://github.com/kubernetes-sigs/sig-windows-dev-tools)
- [Windows host-process containers](https://www.youtube.com/watch?v=fSmDmwKwFfQ)
- [Alpha in v1.22: Windows HostProcess](Containers https://kubernetes.io/blog/2021/08/16/windows-hostprocess-containers/)
- [Create a Windows HostProcess Pod](https://kubernetes.io/docs/tasks/configure-pod-container/create-hostprocess-pod/)
- [Introducing the Host Compute Service (HCS)](https://techcommunity.microsoft.com/t5/containers/introducing-the-host-compute-service-hcs/ba-p/382332)
- [Golang interface for using the Windows Host Compute Service (HCS)](https://github.com/microsoft/hcsshim)




