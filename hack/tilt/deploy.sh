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

KPNG_IMAGE="kpng"
function build_kpng_and_load_image {
  if docker build -t $KPNG_IMAGE:latest $SRC_DIR/ ; then
	  echo "passed build"
  else
	  echo "failed build"
    exit 123
  fi
  $BIN_DIR/kind load docker-image kpng --name kpng-proxy
}

if ! [[ "${backend}" =~ ^(iptables|nft|ipvs|ebpf|userspacelin|not-kpng)$ ]]; then
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

build_kpng_and_load_image

rm -rf ${SCRIPT_DIR}/kpng-deployment-ds.yaml
kubectl -n ${namespace} delete ds kpng 

go run ${SCRIPT_DIR}/kpng-ds-yaml-gen.go ${SCRIPT_DIR}/kpng-deployment-ds-template.txt ${SCRIPT_DIR}/kpng-deployment-ds.yaml

${BIN_DIR}/kubectl apply -f ${SCRIPT_DIR}/kpng-deployment-ds.yaml
