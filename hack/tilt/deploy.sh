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

TILT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
SCRIPT_DIR=$(dirname $TILT_DIR)
SRC_DIR=$(dirname $SCRIPT_DIR)
BIN_DIR=$(dirname $SCRIPT_DIR)/temp/tilt/bin 

source ${SCRIPT_DIR}/utils.sh
source ${SCRIPT_DIR}/common.sh 

# system data
# overwrrite the default name-space(kube-system)
NAMESPACE="tilt-dev" 
CONFIG_MAP_NAME="kpng"
SERVICE_ACCOUNT_NAME="kpng"
CLUSTER_ROLE_NAME="system:node-proxier"
CLUSTER_ROLE_BINDING_NAME="kpng"
KPNG_IMAGE="kpng"



# Specify the default values for generating ds yaml.
function export_vars {
: ${kpng_image:=kpng}; export kpng_image
: ${image_pull_policy:=IfNotPresent}; export image_pull_policy
: ${backend:=nft}; export backend
: ${config_map_name:=kpng}; export config_map_name
: ${service_account_namee:=kpng}; export service_account_name
: ${namespace:=kube-system}; export namespace
: ${e2e_backend_args:=['kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', '--exportMetrics=0.0.0.0:9099', 'to-local', 'to-nft', '--v=4', '--cluster-cidrs=fd6d:706e:6701::/56']}; export e2e_backend_args
: ${e2e_server_args:=['kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', '--exportMetrics=0.0.0.0:9099', 'to-local', 'to-nft', '--v=4', '--cluster-cidrs=fd6d:706e:6701::/56']}; export e2e_server_args
: ${deployment_model:=single-process-per-node}; export deployment_model
}

function build_kpng_and_load_image {
  if docker build -t $KPNG_IMAGE:latest $SRC_DIR/ ; then
	  echo "passed build"
  else
	  echo "failed build"
    exit 123
  fi
  $BIN_DIR/kind load docker-image kpng --name kpng-proxy
}


if ! [[ "${backend}" =~ ^(iptables|nft|ipvs|ipvsfullstate|ebpf|userspacelin|not-kpng)$ ]]; then
  echo "skipping re-deployment due to unsupported backend"
  exit 0 
fi

add_to_path "${BIN_DIR}"

if [ "${backend}" == "ebpf" ] ; then
  if [ "${ip_family}" != "ipv4" ] ; then
  echo "skipping re-deployment. ebpf backend only supports ipv4"
    exit 0
  fi
  compile_bpf
fi

SERVER_ARGS="'kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', '--exportMetrics=0.0.0.0:9099', 'to-api', '--listen=unix:///k8s/proxy.sock'"
BACKEND_ARGS="'local', '--api=${KPNG_SERVER_ADDRESS}', '--exportMetrics=0.0.0.0:9098', 'to-${backend}', '--v=${KPNG_DEBUG_LEVEL}'"
    
if [[ "${backend}" == "nft" ]]; then
  case $ip_family in
    ipv4 ) BACKEND_ARGS="$BACKEND_ARGS, '--cluster-cidrs=${CLUSTER_CIDR_V4}'" ;;
    ipv6 ) BACKEND_ARGS="$BACKEND_ARGS, '--cluster-cidrs=${CLUSTER_CIDR_V6}'" ;;
    dual ) BACKEND_ARGS="$BACKEND_ARGS, '--cluster-cidrs=${CLUSTER_CIDR_V4}', '--cluster-cidrs=${CLUSTER_CIDR_V6}'" ;;
  esac
fi

BACKEND_ARGS="[$BACKEND_ARGS]"
SERVER_ARGS="[$SERVER_ARGS]" 
export e2e_backend_args=$BACKEND_ARGS
export e2e_server_args=$SERVER_ARGS

build_kpng_and_load_image

rm -rf ${SCRIPT_DIR}/kpng-deployment-ds.yaml
kubectl -n ${namespace} delete ds kpng 

export_vars 

go run ${SCRIPT_DIR}/kpng-ds-yaml-gen.go ${SCRIPT_DIR}/kpng-deployment-ds-template.txt ${SCRIPT_DIR}/kpng-deployment-ds.yaml

${BIN_DIR}/kubectl apply -f ${SCRIPT_DIR}/kpng-deployment-ds.yaml