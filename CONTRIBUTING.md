# Contributing to KPNG

## Join the team

Since KPNG has a very lofty goal, replacing the kube-proxy, working together is important.
We pair program at our meetings and in general, encourage people to own large problems, end to end.

## What type of contributions are needed 

We need feature owners, for specific backends especially, which fix features that will bring KPNG to parity
with the upstream kube proxy. We also need people to fix e2e tests for exisitng backends.

## How to get started?

There are many ways to get started, but here's a good set of guidelines.

### First, make sure you understand the basics of K8s networking.

In particular, you should be able to differentiate:
- CNI providers (i.e. calico, antrea, cilium, and so on)
- Service Proxies (i.e. kube-proxy, AntreaProxy, CiliumProxy, various service-mesh's, and so on)
- LoadBalancers (i.e. serviceType=LoadBalancer)

There are about 20 or 30 great youtube videos about Kubernetes networking and the Kube proxy that you can easily search for and study.
There are also several books about Kubernetes networking and basic Kubernetes architecture.

### Next, skim the Kube proxy codebase

The existing K8s codebase https://github.com/kubernetes/kubernetes/tree/master/pkg/proxy, has a complex, monolithic, battle-hardened proxy.
Read through it and try to put the peices together, so you understand the overall problem space that KPNG solves.

### Get KPNG up and running with Tilt

To quickly get a developer setup and do some experiments, Tilt setup can be used. You need to install

- [Tilt](https://docs.tilt.dev/install.html)
- Docker

If Tilt and Docker are already available, you can create a Kind cluster for Tilt using `make tilt-setup`. It requires arguments, IPfamily and backend. It’s possible to change the backend and re-deploy the changes without creating a new kind cluster.

Eg:

```console
make tilt-setup b=ebpf i=ipv4
```

Once the Kind cluster is ready, You can start the Tilt server by executing `make tilt-up`.

The `make tilt-setup` command will create a file with name `tilt.env`, this file will contain environment variables required to generate kpng DaemonSet yaml. You can change the backend from the `tilt.env` file and it’ll trigger a redeployment. The below is a sample `tilt.env` file. However, some changes (eg: `ip_family`) won't trigger re-deployment(`ip_family` change requires to create a new cluster).

Example `tilt.env` file can be found in `hack/tilt/tilt.env.example`.

```console
kpng_image=kpng
image_pull_policy=IfNotPresent
backend=iptables
config_map_name=kpng
service_account_name=kpng
namespace=kube-system
e2e_backend_args=['local', '--api=unix:///k8s/proxy.sock', '--exportMetrics=0.0.0.0:9098', 'to-iptables', '--v=4']
e2e_server_args=['kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', '--exportMetrics=0.0.0.0:9099', 'to-api', '--listen=unix:///k8s/proxy.sock']
deployment_model=single-process-per-node
ip_family=ipv4
```
A video demo to use Tilt can be seen [here](https://drive.google.com/file/d/1UgNPanmmXplY_rKyH0jz21PuzdJmUg7R/view?usp=share_link)

### Now, join the KPNG meetings!

Now that you've seen the basics, you're ready to join the KPNG group meetings and find a project to get involved with!

## I tried to get started, but I can't get it working

That's fine. We move fast and things get broken sometimes as a result. Please file an issue describing what you
did and what happened vs what you expected to happen. Furthermore, if you want to take a look at fixing the issue
yourself, its a great way to understand the codebase and KPNG's architecture.
You can always send a message on the #sig-network-kpng channel on Kubernetes Slack if you need any help.

## Can i add a new backend? 

Sure! if you plan on owning it. Right now we don't know what backends will and won't make it into the standard
k8s proxy, but, in general we have an examples/ and backends/ directory both of which can be used to hold
a new KPNG implementation. Join the india or USA pairing sessions (wednesdays, fridays) to discuss your backend.

Worst case, due to KPNGs pluggable architecture, you can do what lars did and just make a blog post with a link to your
external backend, and vendor KPNG in where needed for testing/running...

https://kubernetes.io/blog/2021/10/18/use-kpng-to-write-specialized-kube-proxiers/

## What about minutia?

Reviewer bandwidth, rebases, and so on are really alot of work.

Large commits which fix obvious glaring holes in the dataplane of KPNG are what we need at this time.

See Kelsey's notes on minutia :

https://github.com/kelseyhightower/kubernetes-the-hard-way/blob/master/CONTRIBUTING.md#notes-on-minutiae

