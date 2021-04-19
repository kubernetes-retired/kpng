#!/bin/bash
# build the kpng image...

IMAGE="jayunit100/kpng-server:latest"

function setup_k8s {
	# make a gopath if not one existing...
	if "$GOPATH" == "" ; then
		mkdir -p $HOME/go/
		export GOPATH=$HOME/go
		# need kind 0.11 bc 0.10 has a bug w/ kubeproxy=none
		GO111MODULE="on" go get sigs.k8s.io/kind@master
		chmod 755 $GOPATH/bin/kind
		cp $GOPATH/bin/kind /usr/local/bin/kind
	fi
	kind version
	kind create cluster --config kind.yaml
}

function build {
	cd ../

	docker build -t $IMAGE ./
	docker push $IMAGE

	cd hack/
}

function install {
	# substitute it with your changes...
	cat kpng-deployment-ds.yaml.tmpl | sed "s,KPNG_IMAGE,$IMAGE," > kpng-deployment-ds.yaml
	kubectl create sa kube-proxy -n kube-system
	kubectl delete -f kpng-deployment-ds.yaml
	kubectl create -f kpng-deployment-ds.yaml
}

# Comment out build if you just want to install the default, i.e. for quickly getting up and running.
#setup_k8s
# build
install
