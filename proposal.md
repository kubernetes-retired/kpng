
Draft of goals:

* build an intermediate model specific to represent the expected node-local state (ENLS)
* digest API server's changes to the ENLS
  * allows to trigger changes only when the ENLS changes (not on every API event)
  * naturally decouples and factorizes proxy-specific logic
* ENLS should be serializable in protobuf, and may be available through an API (internal?)
  * should ease debugging
  * should ease independant updates on each side of this API (k8s semantics before ENLS, subsystems after)
* rework current proxy modes as "plugins"
* define APIs or something to aggregate publications to subsystems like ipvs or iptables from multiple vendors
  * aggregating allows to reduce the syscall load and to factorize update logic (ie: rate limits)
  * ie: calico iptables rules could be pushed to the proxy, eliminating lock contention
* have a framework approach to ease implementations of more autonomous "kube-proxies" (iptables or ipvs only, nftables, eBPF...)

This probably means a lot a reimplementation, going from scratch and trying to merge current code, so starting in a new repository seems to make sense. It may be moved to kubernetes/staging later if preferred.

## Example of event frequency reduction made by diff'ing the ENLS

rev = real changes propagated to listener

event = number of API events received

On an empty KinD cluster:

```
I0122 17:31:28.028679 1254706 event-handlers.go:41] -- event 1, rev 0, revs/events=0%
I0122 17:31:28.028704 1254706 event-handlers.go:41] -- event 2, rev 0, revs/events=0%
I0122 17:31:28.028717 1254706 event-handlers.go:41] -- event 3, rev 0, revs/events=0%
I0122 17:31:28.028726 1254706 event-handlers.go:41] -- event 4, rev 0, revs/events=0%
I0122 17:31:28.028753 1254706 event-handlers.go:41] -- event 5, rev 0, revs/events=0%
I0122 17:31:28.029035 1254706 correlator.go:149] endpoints update: Namespace:"default" Name:"kubernetes" Type:"ClusterIP" IPs:<ClusterIP:"10.96.0.1" > Ports:<Name:"https" Protocol:TCP Port:443 TargetPort:6443 > Subsets:<Ports:<Name:"https" Protocol:TCP Port:6443 > IPsV4:"10.234.0.2" > AllIPsV4:"10.234.0.2"
I0122 17:31:28.029064 1254706 correlator.go:161] all informers are synced
I0122 17:31:28.029073 1254706 event-handlers.go:41] -- event 6, rev 1, revs/events=16%
I0122 17:31:28.029138 1254706 correlator.go:149] endpoints update: Namespace:"kube-system" Name:"kube-dns" Type:"ClusterIP" IPs:<ClusterIP:"10.96.0.10" > Ports:<Name:"dns" Protocol:UDP Port:53 TargetPort:53 > Ports:<Name:"dns-tcp" Protocol:TCP Port:53 TargetPort:53 > Ports:<Name:"metrics" Protocol:TCP Port:9153 TargetPort:9153 > Subsets:<Ports:<Name:"metrics" Protocol:TCP Port:9153 > Ports:<Name:"metrics" Protocol:TCP Port:9153 > Ports:<Name:"metrics" Protocol:TCP Port:9153 > IPsV4:"10.244.0.2" IPsV4:"10.244.0.3" > AllIPsV4:"10.244.0.2" AllIPsV4:"10.244.0.3"
I0122 17:31:28.029147 1254706 event-handlers.go:41] -- event 7, rev 2, revs/events=28%
I0122 17:31:28.111528 1254706 event-handlers.go:41] -- event 8, rev 2, revs/events=25%
I0122 17:31:28.960892 1254706 event-handlers.go:41] -- event 9, rev 2, revs/events=22%
I0122 17:31:29.002326 1254706 event-handlers.go:41] -- event 10, rev 2, revs/events=20%
...
I0122 18:07:35.201162 1263406 event-handlers.go:46] -- event 371, rev 4, revs/events=1%
I0122 18:07:37.251036 1263406 event-handlers.go:46] -- event 374, rev 4, revs/events=1%
^C
real 36m10.553s
user  0m01.836s
sys	  0m00.481s
```

=> `(1.836+0.481)/(36*60+10.553) => 1.07%`

On a bigger cluster (1k services, 1.5k pods):

```
I0123 15:41:51.368492   15541 event-handlers.go:46] -- event 1, rev 0, revs/events=0%
... (initial rush) ...
I0123 15:41:52.389062   15541 event-handlers.go:46] -- event 2137, rev 1065, revs/events=49%
I0123 15:41:53.560916   15541 event-handlers.go:46] -- event 2139, rev 1065, revs/events=49%
I0123 15:41:54.754649   15541 event-handlers.go:46] -- event 2141, rev 1065, revs/events=49%
...
I0123 15:53:13.321570   15541 event-handlers.go:46] -- event 50149, rev 1065, revs/events=2%
I0123 15:53:15.343342   15541 event-handlers.go:46] -- event 50154, rev 1065, revs/events=2%
...
I0123 16:11:48.897208   15541 event-handlers.go:46] -- event 131074, rev 1211, revs/events=0%
I0123 16:11:50.256433   15541 event-handlers.go:46] -- event 131076, rev 1211, revs/events=0%
^C
real	30m0.006s
user	0m4.705s
sys	0m0.536s
```

=> `(4.705+0.536)/(30*60+0.006) => 2.91%`

