#!/bin/bash
# build the kpng image...

IMAGE="jayunit100/kpng-server:latest"

function build {
	cd ../

	docker build -t $IMAGE ./
	docker push $IMAGE

	cd hack/
}

function install {
	# substitute it with your changes...
	cat kpng-deployment-ds.yaml.tmpl | sed "s,KPNG_IMAGE,$IMAGE," > kpng-deployment-ds.yaml
	kubectl delete -f kpng-deployment-ds.yaml
	kubectl create -f kpng-deployment-ds.yaml
}

# Comment out build if you just want to install the default, i.e. for quickly getting up and running.
build
install
