
# Scripts and Vagrantfile to Automate Kubernetes cluster using Kubeadm for demo and testing pf KPNG

## Documentation

Refer this link for documentation: https://devopscube.com/kubernetes-cluster-vagrant/

If you are preparing for CKA, CKAD or CKS exam, save 15% using code **SCOFFER15** at https://kube.promo/latest

## Prerequisites

1. Vagrant installed
2. 8 Gig + RAM and3vCP
U 
## Utility

To provision the cluster, execute the following commands.

```shell
git clone https://github.com/kubernetes-sigs/kpng.git
cd kpng/vagrant_kpng
vagrant up
```

## Set Kube-Proxy mode.

```shell
kubectl -n kube-system edit configmap kube-proxy

change mode:
	userspace	
	ipvs
        iptables
```

## Kubernetes Dashboard URL

```shell
http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/#/overview?namespace=kubernetes-dashboard
```

## CNI support

At present only Calico support present.
Support for all CNIs will be added as command line argument input


## To shutdown the cluster, 

```shell
vagrant halt
```

## To restart the cluster,

```shell
vagrant up
```

## To destroy the cluster, 

```shell
vagrant destroy -f
```

  
