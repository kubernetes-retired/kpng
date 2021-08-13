#!/bin/bash
# build the kpng image...

# TODO Replace with 1.22 once we address 
#: ${KIND:="kindest/node:v1.21.1@sha256:69860bda5563ac81e3c0057d654b5253219618a22ec3a346306239bba8cfa1a6"}
: ${KIND:="kindest/node:v1.22.0@sha256:b8bda84bb3a190e6e028b1760d277454a72267a5454b57db34437c34a588d047"}
: ${IMAGE:="jayunit100/kpng:ipvs"}
: ${PULL:=IfNotPresent}
: ${BACKEND:=nft}

export IMAGE PULL BACKEND

echo -n "this will deploy kpng with docker image $IMAGE, pull policy $PULL and the $BACKEND backend. Press enter to confirm, C-c to cancel"
read

function install_calico {
    kubectl apply -f https://raw.githubusercontent.com/jayunit100/k8sprototypes/master/kind/calico.yaml
    kubectl -n kube-system set env daemonset/calico-node FELIX_IGNORELOOSERPF=true
    kubectl -n kube-system set env daemonset/calico-node FELIX_XDPENABLED=false
}

function setup_k8s {
    kind version

    echo "****************************************************"
    kind delete cluster --name kpng-proxy
    kind create cluster --config kind.yaml --image $KIND
    install_calico
    echo "****************************************************"
}

function build {
    cd ../

    docker build -t $IMAGE ./
    docker push $IMAGE

    cd hack/
}

function install {
    # substitute it with your changes...
    echo "Applying template"
    envsubst <kpng-deployment-ds.yaml.tmpl >kpng-deployment-ds.yaml

    kind load docker-image $IMAGE --name kpng-proxy

    kubectl -n kube-system create sa kpng
    kubectl create clusterrolebinding kpng --clusterrole=system:node-proxier --serviceaccount=kube-system:kpng
    kubectl -n kube-system create cm kpng --from-file kubeconfig.conf

    kubectl delete -f kpng-deployment-ds.yaml
    kubectl create -f kpng-deployment-ds.yaml
}

# Comment out build if you just want to install the default, i.e. for quickly getting up and running.
setup_k8s
build
install
