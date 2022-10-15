# Contributing to KPNG

## Change something and see if it works

Just run
```
make test
```

Then try running:
```
./test_e2e.sh -i ipv4 -b iptables
```

you'll see a running kind cluster, and you can see what conformance and sig-net tests pass/fail.

## Join the team

Since KPNG has a very lofty goal, replacing the kube-proxy, working together is important.
We pair program at our meetings and in general, encourage people to own large problems, end to end.

## What type of contributions are needed 

We need feature owners, for specific backends especially, which fix features that will bring KPNG to parity
with the upstream kube proxy.

## Can i add a new backend? 

Sure ! if you plan on owning it.  Right now we don't know what backends will and won't make it into the standard
k8s proxy, but, in general we have an examples/ and backends/ directory both of which can be used to hold
a new KPNG implementation  .  Join the india or USA pairing sessions (wednesdays, fridays) to discuss your backend.

Worst case, due to KPNGs pluggable architecture, you can do what lars did and just make a blog post with a link to your
external backend, and vendor KPNG in where needed for testing/running...

https://kubernetes.io/blog/2021/10/18/use-kpng-to-write-specialized-kube-proxiers/

## What about minutia?

Reviewer bandwidth, rebases, and so on are really alot of work.

Large commits which fix obvious glaring holes in the dataplane of KPNG are what we need at this time.

See Kelsey's notes on minutia :

https://github.com/kelseyhightower/kubernetes-the-hard-way/blob/master/CONTRIBUTING.md#notes-on-minutiae

