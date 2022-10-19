# Interface Print State Example

This Example showcases the use of KPNG as a library, which simply prints the
ENLS(expected node local state) for services and endpoints from a running
K8s cluster.

To run this manually simply point it to a kubeconfig and specify the node name
you're interested in i.e

```bash
go run main.go --kubeconfig=/home/USER/.kube/config --nodename=kpng-e2e-ipv4-nft-control-plane
```

And you can watch as the state is delivered.

For example when a simple hello world service is created with

`kubectl expose deployment nginx-deployment --type=ClusterIP`

The example shows the expected changes for a given node:

```bash
go run main.go --kubeconfig=/home/astoycos/.kube/config --nodename=kpng-e2e-ipv4-nft-worker
...
I1021 15:06:50.966214 1033987 main.go:37] Got Service Update: Name -> nginx-deployment, Namespace -> default
I1021 15:06:50.970474 1033987 main.go:53] Got Endpoint Update: [EPSName: nginx-deployment-qzxrf, Ips: V4:"10.1.1.7", isLocal: true] for Service: nginx-deployment 
I1021 15:06:50.970576 1033987 main.go:53] Got Endpoint Update: [EPSName: nginx-deployment-qzxrf, Ips: V4:"10.1.2.8", isLocal: false] for Service: nginx-deployment 
```
