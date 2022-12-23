#!/bin/bash

# Copyright 2021 The Kubernetes Authors.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#     http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source "${SCRIPT_DIR}/utils.sh"
source "${SCRIPT_DIR}/common.sh"

# build the kpng image...
: ${KIND:="kindest/node:v1.22.13@sha256:4904eda4d6e64b402169797805b8ec01f50133960ad6c19af45173a27eadf959"}
: ${IMAGE:="gauravkghildiyal/kpng:latest"}
: ${PULL:="IfNotPresent"}
: ${BACKEND:="nft"}
: ${CONFIG_MAP_NAME:=kpng}
: ${SERVICE_ACCOUNT_NAME:=kpng}
: ${NAMESPACE:=kube-system}
: ${KPNG_DEBUG_LEVEL:=4}
: ${BACKEND_ARGS:="['local', '--api=unix:///k8s/proxy.sock', 'to-${BACKEND}', '--v=${KPNG_DEBUG_LEVEL}']"}

echo -n "this will deploy kpng with docker image $IMAGE, pull policy '$PULL' and the '$BACKEND' backend. Press enter to confirm, C-c to cancel"
read

function build_kpng {
    cd ../

    if docker build -t $IMAGE ./ ; then
	echo "passed build"
    else
	echo "failed build"
        exit 123
    fi
    docker push $IMAGE
    cd hack/
}


function install_k8s {
    kind version

    echo "****************************************************"
    kind delete cluster --name kpng-proxy
    kind create cluster --config kind.yaml --image $KIND
    echo "****************************************************"
}

function install_kpng {
    # substitute it with your changes...

    echo "Applying template"
    # Setting vars for generate the kpng deployment based on template
    export kpng_image="${IMAGE}" 
    export image_pull_policy="${PULL}" 
    export backend="${BACKEND}" 
    export config_map_name="${CONFIG_MAP_NAME}" 
    export service_account_name="${SERVICE_ACCOUNT_NAME}" 
    export namespace="${NAMESPACE}" 
    export e2e_backend_args="${BACKEND_ARGS}"
    export e2e_server_args="['kube','--kubeconfig=/var/lib/kpng/kubeconfig.conf', '--exportMetrics=0.0.0.0:9099', 'to-api']"

    go run kpng-ds-yaml-gen.go ./kpng-deployment-ds-template.txt ./kpng-deployment-ds.yaml
    if_error_exit "error generating kpng deployment YAML"

    kind load docker-image $IMAGE --name kpng-proxy

    # todo(jayunit100) support antrea as a secondary CNI option to test

    kubectl -n kube-system create sa kpng
    kubectl create clusterrolebinding kpng --clusterrole=system:node-proxier --serviceaccount="${NAMESPACE}:${SERVICE_ACCOUNT_NAME}"
    kubectl -n kube-system create cm kpng --from-file kubeconfig.conf

    # Deleting any previous daemonsets.apps kpng
    kubectl delete -f kpng-deployment-ds.yaml 2> /dev/null
    kubectl create -f kpng-deployment-ds.yaml
    rm -rf kpng-deployment-ds.yaml
}

# cd to dir of this script
cd "${0%/*}"

# Comment out build if you just want to install the default, i.e. for quickly getting up and running.
build_kpng
install_k8s
install_kpng
