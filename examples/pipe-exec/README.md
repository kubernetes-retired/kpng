# KPNG example

This is an example for a Kubernetes [blog
post](https://kubernetes.io/blog/2021/10/18/use-kpng-to-write-specialized-kube-proxiers/).

The [kpng](https://github.com/kubernetes-sigs/kpng) project provides a
simple way to write a customized proxier that can live side by side
with the standard `kube-proxy`. This example uses a simple script
callout backend to setup a "all-ip" external service.


## Manual testing

Binaries for `kpng` (snapshot) and the backends `kpng-json` and
`kpng-callout` can be
[downloaded](https://storage.googleapis.com/jayunit100/lars-kpng-proxy-blog-9-2021.tar.xz).

Simple printout:
```
kubectl apply -f ./manifests/kpng-example-app.yaml
kpng kube --service-proxy-name=kpng-example to-api &
kpng-json
```

This can be done outside the cluster.

**NOTE:** At the time of writing `kpng` does not support loadBalancer
addresses so `ExternalIPs` must be used in the service.



## The example kpng proxier

**NOTE**: A privileged `kpng` controller with `hostNetwork: true` is started on
all nodes (DaemonSet). Do this in experimental clusters only!


```
kubectl apply -f ./manifests/kpng-example-proxy.yaml
kubectl apply -f ./manifests/kpng-example-app.yaml
# Then on a node;
ip6tables -t nat -S PREROUTING
iptables -t nat -S PREROUTING
nc 1000::55 6000
# Outside the cluster (with routing setup correctly)
nc 1000::55 6000
```

### How it works

1. The `kpng-example-proxy` POD is started with the [kpng-example-start](scripts/kpng-example-start) script

2. The `kpng` controller and the `kpng-callout` backend are started

3. The service json data is read by the `kpng` controller and passed to the `kpng-callout` backend

4. The `kpng-callout` backend starts the program pointed out by the "$CALLOUT" variable and passes the json data on stdin

5. The [kpng-example-allip](scripts/kpng-example-allip) script interpretes the json data and setup iptables rules



## Build

Build the backends:
```
make binaries
```

Build the image:
```
make binaries
docker build -t your.tag.here .
```

You may want to use a `kpng` image that you have built yourself as a
base. Edit the "FROM" entry in the `Dockerfile`. You can build the
`kpng` image with:

```
cd /your/path/to/a/cloned/kpng
docker build -t your.kpng.tag.here .
```
