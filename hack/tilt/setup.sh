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
BIN_DIR=$(dirname $SCRIPT_DIR)/temp/tilt/bin 

KIND="kindest/node:v1.22.13@sha256:4904eda4d6e64b402169797805b8ec01f50133960ad6c19af45173a27eadf959"

source ${SCRIPT_DIR}/utils.sh 
source ${SCRIPT_DIR}/common.sh 

# overwrrite the default name-space(kube-system)
NAMESPACE="tilt-dev"

echo $NAMESPACE

function create_cluster {
    ###########################################################################
    # Description:                                                            #
    # create cluster                                          #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: ip_family                                                       #
    ###########################################################################
    [ $# -eq 1 ]
    if_error_exit "set_host $# Wrong number of arguments to ${FUNCNAME[0]}"
    local ip_family="${1}"
    
    ${BIN_DIR}/kind version

    echo "****************************************************"
    ${BIN_DIR}/kind delete cluster --name ${CLUSTER_NAME}

    case $ip_family in
        ipv4 )
            CLUSTER_CIDR="${CLUSTER_CIDR_V4}"
            SERVICE_CLUSTER_IP_RANGE="${SERVICE_CLUSTER_IP_RANGE_V4}"
            ;;
        ipv6 )
            CLUSTER_CIDR="${CLUSTER_CIDR_V6}"
            SERVICE_CLUSTER_IP_RANGE="${SERVICE_CLUSTER_IP_RANGE_V6}"
            ;;
        dual )
            CLUSTER_CIDR="${CLUSTER_CIDR_V4},${CLUSTER_CIDR_V6}"
            SERVICE_CLUSTER_IP_RANGE="${SERVICE_CLUSTER_IP_RANGE_V4},${SERVICE_CLUSTER_IP_RANGE_V6}"
            ;;
    esac
    
    # create the config file
cat <<EOF >hack/tilt/kind.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
    kubeProxyMode: "none"
    apiServerAddress: "0.0.0.0"
    disableDefaultCNI: true
    ipFamily: "${ip_family}"
    podSubnet: "${CLUSTER_CIDR}"
    serviceSubnet: "${SERVICE_CLUSTER_IP_RANGE}"
nodes:
- role: control-plane
- role: worker
- role: worker
EOF

    $BIN_DIR/kind create cluster \
      --name "${CLUSTER_NAME}"                     \
      --image "${KINDEST_NODE_IMAGE}":"${K8S_VERSION}"    \
      --retain \
      --wait=1m \
      --config=hack/tilt/kind.yaml 
    if_error_exit "cannot create kind cluster ${CLUSTER_NAME}"

    $BIN_DIR/kubectl create namespace $NAMESPACE
    $BIN_DIR/kubectl -n $NAMESPACE create sa $SERVICE_ACCOUNT_NAME
    $BIN_DIR/kubectl create clusterrolebinding $CLUSTER_ROLE_BINDING_NAME --clusterrole=$CLUSTER_ROLE_NAME --serviceaccount="${NAMESPACE}:${SERVICE_ACCOUNT_NAME}"
    $BIN_DIR/kubectl -n $NAMESPACE create cm $CONFIG_MAP_NAME --from-file ${SCRIPT_DIR}/kubeconfig.conf
    
    install_calico $BIN_DIR
    echo "****************************************************"
}

function configure_env_vars {
    ###########################################################################
    # Description:                                                            #
    # Configure env variables which will be used in tilt                                           #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: ip_family
    #   arg2: backend
    #   arg3: deployment_model                                                       #
    ###########################################################################

    [ $# -eq 3 ]
    if_error_exit "set_host $# Wrong number of arguments to ${FUNCNAME[0]}"
    local ip_family="${1}"
    local backend="${2}"
    local deployment_model="${3}"

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

cat <<EOF >tilt.env
kpng_image=kpng 
image_pull_policy=IfNotPresent 
backend=${backend}
config_map_name=${CONFIG_MAP_NAME}
service_account_name=${SERVICE_ACCOUNT_NAME}
namespace=${NAMESPACE} 
e2e_backend_args=${BACKEND_ARGS}
e2e_server_args=${SERVER_ARGS}
deployment_model=${deployment_model}
ip_family=${ip_family}
EOF
}

function help {
    ###########################################################################
    # Description:                                                            #
    # Help function to be displayed                                           #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    printf "\n"
    printf "Usage: %s [i=ip_family] [b=backend] [m=deployment_model]\n" "$0"
    printf "\t-m set the KPNG deployment model, can either be: \n\
            * split-process-per-node [legacy/debug] -> (To run KPNG server + client in separate containers/processes per node)
            * single-process-per-node [default] -> (To run KPNG server + client in a single container/process per node)"

    printf "\nExample:\n\t %s i=ipv4 b=iptables\n" "${0}"
    exit 1 # Exit script after printing help
}

function main {
    ###########################################################################
    # Description:                                                            #
    # main function to execute infrastructure setup                                           #
    #                                                                         #
    # Arguments:                                                              #
    #   ip_family                                                               
    #   backend
    #   deployment_model                                                                 #
    ###########################################################################
    [ $# -eq 3 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    # setting up variables
    local ip_family="${1}"
    local backend="${2}"
    local deployment_model="${3}"
    
    
    mkdir -p ${BIN_DIR}/
    # TODO: change os variable
    install_binaries "${BIN_DIR}" "${K8S_VERSION}" "${OS}"

    set_host_network_settings "${ip_family}"

    # ip family verification for ebpf
    if [ "${backend}" == "ebpf" ] ; then
        if [ "${ip_family}" != "ipv4" ] ; then
            echo "ebpf backend only supports ipv4"
            exit 1
        fi
    fi

    verify_host_network_settings "${ip_family}"

    create_cluster "${ip_family}"

    configure_env_vars "${ip_family}" "${backend}" "${deployment_model}"
}

while getopts "i:b:m" flag
do
    case "${flag}" in
        i ) ip_family="${OPTARG}" ;;
        b ) backend="${OPTARG}" ;;
        m ) deployment_model="${OPTARG}" ;;
        ? ) help ;; #Print help
    esac
done

if ! [[ "${backend}" =~ ^(iptables|nft|ipvs|ipvsfullstate|ebpf|userspacelin|not-kpng)$ ]]; then
    echo "user must specify the supported backend"
    help
fi

if ! [[ "${ip_family}" =~ ^(ipv4|ipv6|dual)$ ]]; then
    echo "user must specify the ip family"
    help
fi

if [ ! "$deployment_model" ];then
   deployment_model="single-process-per-node"
fi

if [[ -n "${ip_family}" && -n "${backend}" ]];   then
    if ! [[ "${backend}" =~ ^(iptables|nft|ipvs|ipvsfullstate|ebpf|userspacelin|not-kpng)$ ]]; then
      echo "user must specify the supported backend"
      help  
      exit 1 
    fi

    if ! [[ "${ip_family}" =~ ^(ipv4|ipv6|dual)$ ]]; then
        echo "user must specify the ip family"
        help
        exit 1
    fi
    main "${ip_family}" "${backend}" "${deployment_model}"
else
    printf "Both of 'i' and 'b' must be specified.\n"
    help
fi