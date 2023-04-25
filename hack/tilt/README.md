# How to run tilt on kpng
Here is how to run tilt on kpng:
- First we make sure tilt is installed on out local machine/host, we can confirm that by running `tilt version` command on our CLI, if nothing pops up or there's an error then you have to install tilt on your local machine. Here is the getting [started guide](https://docs.tilt.dev/index.html) to install tilt on your local machine, after installing run `tilt version` to confirm that it is installed on your local machine. PS: make sure [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) is also installed on your local machine.
- Next we run the following command one after the other:
```
#To create Kind cluster for tilt. It requires backend(b), ipfamily (i) and kpng deployment model (m) as args:
make tilt-setup i=ipv4 b=nft m=split-process-per-node

#To start tilt server, server will be running on a link popped up from your CLI eg: http://127.0.0.1:10350/, copy link to your browser to access the tilt UI
#From the UI click the kpng-deployment resource that was created to see the build up of kpng:
make tilt-up

#Check cluster for running kpng pods
kubectl get pods -n tilt-dev

#To Stop tilt server:
make tilt-down

#To stop the kind cluster:
kind delete cluster --name kpng-proxy
```
