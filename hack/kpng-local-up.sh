#!/bin/bash
# build the kpng image...

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
    if [ ! -x kind ]; then
        curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.11.0/kind-linux-amd64
        chmod +x ./kind
    fi
    ./kind version

    echo "****************************************************"
    ./kind delete cluster --name kpng-proxy
    ./kind create cluster --config kind.yaml
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
