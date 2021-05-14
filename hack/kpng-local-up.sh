#!/bin/bash
# build the kpng image...

: ${IMAGE:="jayunit100/kpng-server:latest"}

function install_calico {
    kubectl apply -f https://raw.githubusercontent.com/jayunit100/k8sprototypes/master/kind/calico.yaml
    kubectl -n kube-system set env daemonset/calico-node FELIX_IGNORELOOSERPF=true
    kubectl -n kube-system set env daemonset/calico-node FELIX_XDPENABLED=false
}

function setup_k8s {
    # make a gopath if not one existing...
    if [ "$GOPATH" == "" ] ; then
        mkdir -p $HOME/go/
        export GOPATH=$HOME/go
        # need kind 0.11 bc 0.10 has a bug w/ kubeproxy=none
        GO111MODULE="on" go get sigs.k8s.io/kind@main
        export PATH="$(go env GOPATH)/bin:${PATH}"
    fi
    kind version

    echo "****************************************************"
    kind delete cluster --name kpng-proxy
    kind create cluster --config kind.yaml
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
    echo "Applying template with KPNG_IMAGE=$IMAGE"
    cat kpng-deployment-ds.yaml.tmpl | sed "s,KPNG_IMAGE,$IMAGE," > kpng-deployment-ds.yaml

    kubectl -n kube-system create sa kpng
    kubectl create clusterrolebinding kpng --clusterrole=system:node-proxier --serviceaccount=kube-system:kpng
    kubectl -n kube-system create cm kpng --from-file kubeconfig.conf

    kubectl delete -f kpng-deployment-ds.yaml
    kubectl create -f kpng-deployment-ds.yaml
}

# Comment out build if you just want to install the default, i.e. for quickly getting up and running.
setup_k8s
#build
install
