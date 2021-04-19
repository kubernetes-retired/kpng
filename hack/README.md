# Get up and running w kpng

Run the local-up-kpng.sh script (make sure you have a kind or other cluster ready).

This will run kpng using incluster access to the apiserver, as a daeamonset.

You can test your changes by making sure you run both the `build` and `install` functions in this script.

This is just a first iteration on a dev env for kpng, feel free to edit/add stuff.

# How it works

This development recipe works by using `kind` to spin up a cluster.
However it cant use a vanilla kind recipe because:
- we need to add labels for kpng to know where its running its kube-proxy containers
- we need to add a kube-proxy service account 
- we also need to tolerate the controlplane node so that kpng runs there

thus the recipe has separate 'functions' for each phase of running KPNG.

- setup: setup kind and install it, gopath stuff
- build: compile kpng and push it to a registry
- install: delete the kpng daemonset and redeploy it

# Contribute

This is just an initial recipe for learning how kpng works.  Please contribute updates
if you have ideas to make it better.  

- One example of a starting contribution might be
pushing directly to a local registry and configuring kind to use this reg, so dockerhub
isnt required.  
- Or a `tilt` recipe which hot reloads all kpng on code changes.







