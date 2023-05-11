#!/bin/bash
# shellcheck disable=SC2181,SC2155,SC2128
#
# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source "${SCRIPT_DIR}"/utils.sh
source "${SCRIPT_DIR}"/common.sh
source "${SCRIPT_DIR}"/test_skip_list.sh

if [[ $OSTYPE == 'darwin'* ]]; then
  info_message "The kpng build script only works on linux... Exiting now!"
  exit 1
fi

shopt -s expand_aliases

: "${E2E_GO_VERSION:="1.18.4"}"
: "${E2E_TIMEOUT_MINUTES:=100}"
: "${E2E_K8S_VERSION:="v1.27.1"}"

CONTAINER_ENGINE="docker"
KPNG_IMAGE_TAG_NAME="kpng:test"
KUBECONFIG_TESTS="kubeconfig_tests.conf"

# ginkgo
GINKGO_NUMBER_OF_NODES=25
GINKGO_FOCUS=${GINKGO_FOCUS:-"\[Conformance\]|\[sig-network\]"}
GINKGO_SKIP_TESTS="machinery|Feature|Federation|PerformanceDNS|Disruptive|Serial|LoadBalancer|KubeProxy|GCE|Netpol|NetworkPolicy"
GINKGO_REPORT_DIR="artifacts/reports"
GINKGO_DUMP_LOGS_ON_FAILURE=false
GINKGO_DISABLE_LOG_DUMP=true
GINKGO_PROVIDER="local"

function if_error_warning {
    ###########################################################################
    # Description:                                                            #
    # Validate if previous command failed and show an error msg (if provided) #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1 - error message if not provided, it will just exit               #
    ###########################################################################
    if [ "$?" != "0" ]; then
        if [ -n "$1" ]; then
            RED="\033[91m"
            ENDCOLOR="\033[0m"
            echo -e "[ ${RED}FAILED${ENDCOLOR} ] ${1}"
        fi
    fi
}

function result_message {
    ###########################################################################
    # Description:                                                            #
    # show [FAILED] in red and a message.                                     #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1 - result                                                         #
    #   arg2 - message to output                                              #
    ###########################################################################
    [ $# -eq 2 ]
    if_error_exit "result_message: $# Wrong number of arguments to ${FUNCNAME[0]}... args: $#"

    local result=${1}
    local message="${2}"

    if [ -z "${result}" ]; then
        echo "result_message() requires a message"
        exit 1
    fi
    RED="\033[91m"
    GREEN="\033[92m"    
    ENDCOLOR="\033[0m"

    if [ "${result}" == "0" ] ; then
	echo -e "${GREEN}[SUCCESS!] ${message} succeded!!! ${ENDCOLOR}"
    else
	echo -e "${RED}[FAIL!] ${message} failed!!! ${ENDCOLOR}"
    fi 
}

function detect_container_engine {
    ###########################################################################
    # Description:                                                            #
    # Detect Container Engine, by default it is docker but developers might   #
    # use real alternatives like podman. The project welcome both.            #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # If docker is not available, let's check if podman exists
    ${CONTAINER_ENGINE} &> /dev/null
    if [ "$?" != "0" ]; then
        CONTAINER_ENGINE="podman"
        ${CONTAINER_ENGINE} --help &> /dev/null
        if_error_exit "the e2e tests currently support docker and podman as the container engine. Please install either of them"
    fi
    pass_message "Detected Container Engine: ${CONTAINER_ENGINE}"
}

function container_build {
    ###########################################################################
    # Description:                                                            #
    # build a container image for KPNG                                        #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: Path for E2E installation directory, or the empty string         #
    #   arg2: ci_mode         #
    ###########################################################################
    [ $# -eq 2 ]
    if_error_exit "container_build args $# Wrong number of arguments to ${FUNCNAME[0]}... args: $#"

    local CONTAINER_FILE=${1}
    local ci_mode=${2}

    # Running locally it's not necessary to show all info
    QUIET_MODE="--quiet"
    if [ "${ci_mode}" = true ] ; then
        QUIET_MODE=""
    fi

    [ -f "${CONTAINER_FILE}" ]
    if_error_exit "cannot find ${CONTAINER_FILE}"

    CMD_BUILD_IMAGE=("${CONTAINER_ENGINE} build ${QUIET_MODE} -t ${KPNG_IMAGE_TAG_NAME} -f ${CONTAINER_FILE} .")
    pushd "${0%/*}/.." > /dev/null || exit
        if [ -z "${QUIET_MODE}" ]; then
            ${CMD_BUILD_IMAGE}
        else
            ${CMD_BUILD_IMAGE} &> /dev/null

        fi
        if_error_exit "Failed to build kpng, command was: ${CMD_BUILD_IMAGE}"
    popd > /dev/null || exit

    pass_message "Image build and tag ${KPNG_IMAGE_TAG_NAME} is set."
}

function delete_kind_cluster {
    ###########################################################################
    # Description:                                                            #
    # delete kind cluster                                                     #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: cluster name                                                    #
    #   arg2: bin_dir, path to binary directory                               #
    ###########################################################################
    [ $# -eq 2 ]
    if_error_exit "delete kind $# Wrong number of arguments to ${FUNCNAME[0]}"

    local cluster_name="${1}"
    local bin_dir="${2}"

    [ -d "${bin_dir}" ]
    if_error_exit "Directory \"${bin_dir}\" does not exist"
    
    [ -f "${bin_dir}/kind" ]
    if_error_exit "File \"${bin_dir}/kind\" does not exist"
    
    if "${bin_dir}/kind" get clusters | grep -q "${cluster_name}" &> /dev/null; then
        "${bin_dir}/kind" delete cluster --name "${cluster_name}" &> /dev/null
        if_error_warning "cannot delete cluster ${cluster_name}"

        pass_message "Cluster ${cluster_name} deleted."
    fi
}

function create_cluster {
    ###########################################################################
    # Description:                                                            #
    # Create kind cluster                                                     #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: cluster name                                                    #
    #   arg2: bin_dir, path to binary directory                               #
    #   arg3: IP family                                                       #
    #   arg4: artifacts directory                                             #
    #   arg5: ci_mode                                                         #
    ###########################################################################
    [ $# -eq 5 ]
    if_error_exit "create kind $# Wrong number of arguments to ${FUNCNAME[0]}"

    local cluster_name=${1}
    local bin_dir="${2}"
    local ip_family=${3}
    local artifacts_directory=${4}
    local ci_mode=${5}

    [ -d "${bin_dir}" ]
    if_error_exit "Directory \"${bin_dir}\" does not exist"
    
    [ -f "${bin_dir}/kind" ]
    if_error_exit "File \"${bin_dir}/kind\" does not exist"

    [ -f "${bin_dir}/kubectl" ]
    if_error_exit "File \"${bin_dir}/kubectl\" does not exist"

    # Get rid of any old cluster with the same name.
    if "${bin_dir}/kind" get clusters | grep -q "${cluster_name}" &> /dev/null; then
        "${bin_dir}/kind" delete cluster --name "${cluster_name}" &> /dev/null
        if_error_exit "cannot delete cluster ${cluster_name}"

        pass_message "Previous cluster ${cluster_name} deleted."
    fi

     # Default Log level for all components in test clusters
      local kind_cluster_log_level=${KIND_CLUSTER_LOG_LEVEL:-4}
      local kind_log_level="-v3"
      if [ "${ci_mode}" = true ] ; then
          kind_log_level="-v7"
      fi

      # potentially enable --logging-format
      local scheduler_extra_args="      \"v\": \"${kind_cluster_log_level}\""
      local controllerManager_extra_args="      \"v\": \"${kind_cluster_log_level}\""
      local apiServer_extra_args="      \"v\": \"${kind_cluster_log_level}\""
 #     local kubelet_extra_args="      \"v\": \"${kind_cluster_log_level}\""

      # JSON map injected into featureGates config
 #     local feature_gates='{"AllAlpha":false,"AllBeta":false}'
      # --runtime-config argument value passed to the API server
 #     local runtime_config='{"api/alpha":"false", "api/beta":"false"}'

    if [ -n "$CLUSTER_LOG_FORMAT" ]; then
          scheduler_extra_args="${scheduler_extra_args}\"logging-format\": \"${CLUSTER_LOG_FORMAT}\""
          controllerManager_extra_args="${controllerManager_extra_args}\"logging-format\": \"${CLUSTER_LOG_FORMAT}\""
          apiServer_extra_args="${apiServer_extra_args}\"logging-format\": \"${CLUSTER_LOG_FORMAT}\""
    fi

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

    info_message  "Preparing to setup ${cluster_name} cluster ..."
    # create cluster
    # create the config file
     cat <<EOF > "${artifacts_directory}/kind-config.yaml"
      kind: Cluster
      apiVersion: kind.x-k8s.io/v1alpha4
      networking:
          ipFamily: "${ip_family}"
          podSubnet: "${CLUSTER_CIDR}"
          serviceSubnet: "${SERVICE_CLUSTER_IP_RANGE}"
      nodes:
      - role: control-plane
      - role: worker
      - role: worker
EOF

    # get correct sha for K8S image version
    # using sha values from here https://github.com/kubernetes-sigs/kind/releases
    case ${E2E_K8S_VERSION} in
        v1.27.1 )
            E2E_K8S_SHA="@sha256:9915f5629ef4d29f35b478e819249e89cfaffcbfeebda4324e5c01d53d937b09"
            ;;
        v1.27.0 )
            E2E_K8S_SHA="@sha256:c6b22e613523b1af67d4bc8a0c38a4c3ea3a2b8fbc5b367ae36345c9cb844518"
            ;;
        v1.26.3 )
            E2E_K8S_SHA="@sha256:61b92f38dff6ccc29969e7aa154d34e38b89443af1a2c14e6cfbd2df6419c66f"
            ;;
        v1.25.8 )
            E2E_K8S_SHA="@sha256:00d3f5314cc35327706776e95b2f8e504198ce59ac545d0200a89e69fce10b7f"
            ;;
        v1.24.12 )
            E2E_K8S_SHA="@sha256:1e12918b8bc3d4253bc08f640a231bb0d3b2c5a9b28aa3f2ca1aee93e1e8db16"
            ;;
        v1.23.17 )
            E2E_K8S_SHA="@sha256:e5fd1d9cd7a9a50939f9c005684df5a6d145e8d695e78463637b79464292e66c"
            ;;
        v1.22.17 )
            E2E_K8S_SHA="@sha256:c8a828709a53c25cbdc0790c8afe12f25538617c7be879083248981945c38693"
            ;;
        v1.21.14 )
            E2E_K8S_SHA="@sha256:27ef72ea623ee879a25fe6f9982690a3e370c68286f4356bf643467c552a3888"
            ;;
    esac

    "${bin_dir}/kind" create cluster \
      --name "${cluster_name}"                     \
      --image "${KINDEST_NODE_IMAGE}":"${E2E_K8S_VERSION}""${E2E_K8S_SHA}"    \
      --retain \
      --wait=1m \
      "${kind_log_level}" \
      "--config=${artifacts_directory}/kind-config.yaml"
    if_error_exit "cannot create kind cluster ${cluster_name}"

    # Patch kube-proxy to set the verbosity level
    "${bin_dir}/kubectl" patch -n kube-system daemonset/kube-proxy \
       --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/command/-", "value": "--v='"${kind_cluster_log_level}"'" }]'

    "${bin_dir}/kind" get kubeconfig --internal --name "${cluster_name}" > "${artifacts_directory}/kubeconfig.conf"
    "${bin_dir}/kind" get kubeconfig --name "${cluster_name}" > "${artifacts_directory}/${KUBECONFIG_TESTS}"

    # IPv6 clusters need some CoreDNS changes in order to work in k8s CI:
    # 1. k8s CI doesn´t offer IPv6 connectivity, so CoreDNS should be configured
    # to work in an offline environment:
    # https://github.com/coredns/coredns/issues/2494#issuecomment-457215452
    # 2. k8s CI adds following domains to resolv.conf search field:
    # c.k8s-prow-builds.internal google.internal.
    # CoreDNS should handle those domains and answer with NXDOMAIN instead of SERVFAIL
    # otherwise pods stops trying to resolve the domain.
    #
    if [ "${ip_family}" = "ipv6" ]; then
        local k8s_context="kind-${cluster_name}"
       # Get the current config
        local original_coredns=$("${bin_dir}/kubectl" --context "${k8s_context}" get -oyaml -n=kube-system configmap/coredns)
        info_message "Original CoreDNS config:"
        info_message "${original_coredns}"
        # Patch it
        local fixed_coredns=$(
        printf '%s' "${original_coredns}" | sed \
         -e 's/^.*kubernetes cluster\.local/& internal/' \
         -e '/^.*upstream$/d' \
         -e '/^.*fallthrough.*$/d' \
         -e '/^.*forward . \/etc\/resolv.conf$/d' \
         -e '/^.*loop$/d' \
        )
        info_message "Patched CoreDNS config:"
        info_message "${fixed_coredns}"
        printf '%s' "${fixed_coredns}" | "${bin_dir}/kubectl" --context "${k8s_context}" apply -f -
  fi

    pass_message "Cluster ${cluster_name} is created."
}

function wait_until_cluster_is_ready {
    ###########################################################################
    # Description:                                                            #
    # Wait pods with selector k8s-app=kube-dns be ready and operational       #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: cluster name                                                    #
    #   arg2: bin_dir, path to binary directory                               #
    #   arg2: ci_mode                                                         #
    ###########################################################################

    [ $# -eq 3 ]
    if_error_exit "wait until cluster $# Wrong number of arguments to ${FUNCNAME[0]}"

    local cluster_name="${1}"
    local bin_dir="${2}"
    local ci_mode=${3}
    local k8s_context="kind-${cluster_name}"

    [ -d "${bin_dir}" ]
    if_error_exit "Directory \"${bin_dir}\" does not exist"
    
    [ -f "${bin_dir}/kubectl" ]
    if_error_exit "File \"${bin_dir}/kubectl\" does not exist"

    "${bin_dir}/kubectl" --context "${k8s_context}" wait \
        --for=condition=ready \
        pods \
        --namespace="${NAMESPACE}" \
        --selector k8s-app=kube-dns 1> /dev/null

    if [ "${ci_mode}" = true ] ; then
        "${bin_dir}/kubectl" --context "${k8s_context}" get nodes -o wide
        if_error_exit "unable to show nodes"

        "${bin_dir}/kubectl" --context "${k8s_context}" get pods --all-namespaces
        if_error_exit "error getting pods from all namespaces"
    fi

    pass_message "${cluster_name} is operational."
}

function install_kpng {
    ###########################################################################
    # Description:                                                            #
    # Install KPNG following these steps:                                     #
    #   - removes existing kube-proxy                                         #
    #   - load kpng container image                                           #
    #   - create service account, clusterbinding and configmap for kpng       #
    #   - deploy kpng from the template generated                             #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: cluster name                                                    #
    #   arg2: bin_dir, path to binary directory                               #
    ###########################################################################
    [ $# -eq 2 ]
    if_error_exit "install kpng Wrong number of arguments to ${FUNCNAME[0]}"

    local cluster_name="${1}"
    local bin_dir="${2}"
    local k8s_context="kind-${cluster_name}"

    [ -d "${bin_dir}" ]
    if_error_exit "Directory \"${bin_dir}\" does not exist"

    
    [ -f "${bin_dir}/kubectl" ]
    if_error_exit "File \"${bin_dir}/kubectl\" does not exist"

    # remove kube-proxy
    "${bin_dir}/kubectl" --context "${k8s_context}" delete \
        --namespace "${NAMESPACE}" \
        daemonset.apps/kube-proxy 1> /dev/null
    if_error_exit "cannot delete delete daemonset.apps kube-proxy"
    pass_message "Removed daemonset.apps/kube-proxy."

    # preload kpng image
    # TODO move this to ci:
    # docker load --input kpng-image.tar
    CMD_KIND_LOAD_KPNG_TEST_IMAGE=("kind load docker-image ${KPNG_IMAGE_TAG_NAME} --name ${cluster_name}")
    ${CMD_KIND_LOAD_KPNG_TEST_IMAGE} &> /dev/null
    if_error_exit "error loading image to kind, command was: ${CMD_KIND_LOAD_KPNG_TEST_IMAGE}"
    pass_message "Loaded ${KPNG_IMAGE_TAG_NAME} container image."

    # TODO this should be part of the template
    "${bin_dir}/kubectl" --context "${k8s_context}" create serviceaccount \
        --namespace "${NAMESPACE}" \
        "${SERVICE_ACCOUNT_NAME}" 1> /dev/null
    if_error_exit "error creating serviceaccount ${SERVICE_ACCOUNT_NAME}"
    pass_message "Created service account ${SERVICE_ACCOUNT_NAME}."

    "${bin_dir}/kubectl" --context "${k8s_context}" create clusterrolebinding \
        "${CLUSTER_ROLE_BINDING_NAME}" \
        --clusterrole="${CLUSTER_ROLE_NAME}" \
        --serviceaccount="${NAMESPACE}":"${SERVICE_ACCOUNT_NAME}" 1> /dev/null
    if_error_exit "error creating clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME}"
    pass_message "Created clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME}."

    "${bin_dir}/kubectl" --context "${k8s_context}" create configmap \
        "${CONFIG_MAP_NAME}" \
        --namespace "${NAMESPACE}" \
        --from-file "${artifacts_directory}/kubeconfig.conf" 1> /dev/null
    if_error_exit "error creating configmap ${CONFIG_MAP_NAME}"
    pass_message "Created configmap ${CONFIG_MAP_NAME}."

    # TODO this is hacky we should build the KPNG args one level at a time...i.e 
    # Data source + Flags -> KPNG sink + flags
    E2E_SERVER_ARGS="'kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', 'to-api', '--listen=unix:///k8s/proxy.sock'"
    E2E_BACKEND_ARGS="'local', '--api=${KPNG_SERVER_ADDRESS}', 'to-${E2E_BACKEND}', '--v=${KPNG_DEBUG_LEVEL}'"
    
    if [[ "$E2E_EXPORT_METRICS" == "true" ]]; then 
        info_message "Enabling Metrics"
        E2E_SERVER_ARGS="'kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', '--exportMetrics=0.0.0.0:9099', 'to-api', '--listen=unix:///k8s/proxy.sock'"
        E2E_BACKEND_ARGS="'local', '--api=${KPNG_SERVER_ADDRESS}', '--exportMetrics=0.0.0.0:9098',  'to-${E2E_BACKEND}', '--v=${KPNG_DEBUG_LEVEL}'"
    fi

    if [[ "${E2E_DEPLOYMENT_MODEL}" == "single-process-per-node" ]]; then
        E2E_BACKEND_ARGS="'kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', 'to-local', 'to-${E2E_BACKEND}', '--v=${KPNG_DEBUG_LEVEL}'"
        if [[ "$E2E_EXPORT_METRICS" ]]; then
            info_message "Enabling Metrics"
            E2E_BACKEND_ARGS="'kube', '--kubeconfig=/var/lib/kpng/kubeconfig.conf', '--exportMetrics=0.0.0.0:9099', 'to-local', 'to-${E2E_BACKEND}', '--v=${KPNG_DEBUG_LEVEL}'"
        fi
    fi

    if [[ "${E2E_BACKEND}" == "nft" ]]; then
        case $ip_family in
            ipv4 ) E2E_BACKEND_ARGS="$E2E_BACKEND_ARGS, '--cluster-cidrs=${CLUSTER_CIDR_V4}'" ;;
            ipv6 ) E2E_BACKEND_ARGS="$E2E_BACKEND_ARGS, '--cluster-cidrs=${CLUSTER_CIDR_V6}'" ;;
            dual ) E2E_BACKEND_ARGS="$E2E_BACKEND_ARGS, '--cluster-cidrs=${CLUSTER_CIDR_V4}', '--cluster-cidrs=${CLUSTER_CIDR_V6}'" ;;
        esac
    fi
    E2E_BACKEND_ARGS="[$E2E_BACKEND_ARGS]"
    E2E_SERVER_ARGS="[$E2E_SERVER_ARGS]"

    # Setting vars for generate the kpng deployment based on template
    export kpng_image="${KPNG_IMAGE_TAG_NAME}" 
    export image_pull_policy="IfNotPresent" 
    export backend="${E2E_BACKEND}" 
    export config_map_name="${CONFIG_MAP_NAME}" 
    export service_account_name="${SERVICE_ACCOUNT_NAME}" 
    export namespace="${NAMESPACE}" 
    export e2e_backend_args="${E2E_BACKEND_ARGS}"
    export deployment_model="${E2E_DEPLOYMENT_MODEL}"
    export e2e_server_args="${E2E_SERVER_ARGS}"
    go run "${SCRIPT_DIR}"/kpng-ds-yaml-gen.go "${SCRIPT_DIR}"/kpng-deployment-ds-template.txt  "${artifacts_directory}"/kpng-deployment-ds.yaml
    if_error_exit "error generating kpng deployment YAML"

    "${bin_dir}/kubectl" --context "${k8s_context}" create -f "${artifacts_directory}"/kpng-deployment-ds.yaml 1> /dev/null
    if_error_exit "error creating kpng deployment"

    "${bin_dir}/kubectl" --context "${k8s_context}" --namespace="${NAMESPACE}" rollout status daemonset kpng -w --request-timeout=3m 1> /dev/null
    if_error_exit "timeout waiting kpng rollout"

    pass_message "Installation of kpng is done.\n"
}

function run_tests {
     ###########################################################################
     # Description:                                                            #
     # Execute the tests with ginkgo                                           #
     #                                                                         #
     # Arguments:                                                              #
     #   arg1: e2e directory                                                   #
     #   arg2: bin_dir, path to binary directory                               #
     #   arg3: parallel ginkgo tests boolean                                   #
     #   arg4: ip_family                                                       #
     #   arg5: backend                                                         #
     #   arg6: include specific failed tests                                   #
     ###########################################################################

    [ $# -eq 6 ]
    if_error_exit "exec tests $# Wrong number of arguments to ${FUNCNAME[0]}"

    local e2e_dir="${1}"
    local bin_dir="${2}"
    local parallel="${3}"
    local ip_family="${4}"
    local backend="${5}"
    local include_specific_failed_tests="${6}"
    
    local artifacts_directory="${e2e_dir}/artifacts"

    [ -f "${artifacts_directory}/${KUBECONFIG_TESTS}" ]
    if_error_exit "Directory \"${artifacts_directory}/${KUBECONFIG_TESTS}\" does not exist"

    [ -d "${bin_dir}" ]
    if_error_exit "Directory \"${bin_dir}\" does not exist"
    
    [ -f "${bin_dir}/e2e.test" ]
    if_error_exit "File \"${bin_dir}/e2e.test\" does not exist"

    [ -f "${bin_dir}/ginkgo" ]
    if_error_exit "File \"${bin_dir}/ginkgo\" does not exist"

   # ginkgo regexes
   local ginkgo_skip="${GINKGO_SKIP_TESTS:-}"
   local ginkgo_focus=${GINKGO_FOCUS:-"\\[Conformance\\]"}
   # if we set PARALLEL=true, skip serial tests set --ginkgo-parallel
   if [ "${parallel}" = "true" ]; then
     export GINKGO_PARALLEL=y
     if [ -z "${skip}" ]; then
       ginkgo_skip="\\[Serial\\]"
     else
       ginkgo_skip="\\[Serial\\]|${ginkgo_skip}"
     fi
   fi

   if [ "${include_specific_failed_tests}" == "false" ] ; then
       # find ip_type and backend specific skip sets
       skip_set_name="GINKGO_SKIP_${ip_family}_${backend}_TEST"
       declare -n skip_set_ref=${skip_set_name}
       if ! [ -z "${skip_set_ref}" ] ; then
	   ginkgo_skip="${ginkgo_skip}|${skip_set_ref}"
       fi
   fi

   # setting this env prevents ginkgo e2e from trying to run provider setup
   export KUBERNETES_CONFORMANCE_TEST='y'
   # setting these is required to make RuntimeClass tests work ... :/
   export KUBE_CONTAINER_RUNTIME=remote
   export KUBE_CONTAINER_RUNTIME_ENDPOINT=unix:///run/containerd/containerd.sock
   export KUBE_CONTAINER_RUNTIME_NAME=containerd
   
   "${bin_dir}/ginkgo" --nodes="${GINKGO_NUMBER_OF_NODES}" \
           --focus="${ginkgo_focus}" \
           --skip="${ginkgo_skip}" \
           "${bin_dir}/e2e.test" \
           -- \
           --kubeconfig="${artifacts_directory}/${KUBECONFIG_TESTS}" \
           --provider="${GINKGO_PROVIDER}" \
           --dump-logs-on-failure="${GINKGO_DUMP_LOGS_ON_FAILURE}" \
           --report-dir="${GINKGO_REPORT_DIR}" \
           --disable-log-dump="${GINKGO_DISABLE_LOG_DUMP}"
   return $?
}

function clean_artifacts {
    ###########################################################################
    # Description:                                                            #
    # Clean all artifacts and export kind logs                                #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: Path for E2E installation directory                             #
    #   arg2: bin_dir, path to binary directory                               #
    ###########################################################################

    [ $# -eq 2 ]
    if_error_exit "clean artifacts $# Wrong number of arguments to ${FUNCNAME[0]}"

    local e2e_dir="${1}"
    local bin_dir="${2}"
    local log_dir="${E2E_LOGS:-${e2e_dir}/artifacts/logs}"

    [ -d "${bin_dir}" ]
    if_error_exit "Directory \"${bin_dir}\" does not exist"
    
    [ -f "${bin_dir}/kind" ]
    if_error_exit "File \"${bin_dir}/kind\" does not exist"

    "${bin_dir}/kind" export \
       logs \
          --name="$(cat "${e2e_dir}/clustername")" \
          "${log_dir}"
    if_error_exit "cannot export kind logs"

    #TODO in local mode to avoid the overwriting of artifacts from test to test
    #     add logic to this function that moves the content of artifacts
    #     to a dir named clustername+date+time
    pass_message "make sure to safe your result before the next run."
}

function set_e2e_dir {
    ###########################################################################
    # Description:                                                            #
    # Set E2E directory                                                       #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: Path for E2E installation directory                             #
    #   arg2: binary directory, path to where ginko will be installed         #
    ###########################################################################

    [ $# -eq 2 ]
    if_error_exit "set e2e $# Wrong number of arguments to ${FUNCNAME[0]}"

    local e2e_dir="${1}"
    local bin_dir="${2}"

    [ -d "${bin_dir}" ]
    if_error_exit "Directory \"${bin_dir}\" does not exist"

    pushd "${0%/*}" > /dev/null || exit
        mkdir -p "${e2e_dir}"
        mkdir -p "${e2e_dir}/artifacts"
    popd > /dev/null || exit
}

function prepare_container {
    ###########################################################################
    # Description:                                                            #
    # Prepare container                                                       #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: Path of dockerfile                                              #
    #   arg2: ci_mode                                                         #
    ###########################################################################

    [ $# -eq 2 ]
    if_error_exit "prepare_cont $# Wrong number of arguments to ${FUNCNAME[0]}"

    local dockerfile="${1}"
    local ci_mode="${2}"

    # Detect container engine
    detect_container_engine
    container_build "${dockerfile}" "${ci_mode}"
}

function create_infrastructure_and_run_tests {
    ###########################################################################
    # Description:                                                            #
    # create_infrastructure_and_run_tests                                     #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: Path for E2E installation directory                             #
    #   arg2: ip_family                                                       #
    #   arg3: backend                                                         #
    #   arg4: bin_dir                                                         #
    #   arg5: suffix                                                          #
    #   arg6: developer_mode                                                  #
    #   arg7: <ci_mode>                                                       #
    #   arg8: deployment_model                                                #
    #   arg9: export_metrics                                                  #
    #   arg10: include_specific_failed_tests                                  #
    ###########################################################################
    [ $# -eq 10 ]
    if_error_exit "create and run $# Wrong number of arguments to ${FUNCNAME[0]}"

    local e2e_dir="${1}"
    local ip_family="${2}"
    local backend="${3}"
    local bin_dir="${4}"
    local suffix="${5}"
    local devel_mode="${6}"
    local ci_mode="${7}"
    local deployment_model="${8}"
    local export_metrics="${9}"
    local include_specific_failed_tests="${10}"
    

    local artifacts_directory="${e2e_dir}/artifacts"
    local cluster_name="kpng-e2e-${ip_family}-${backend}${suffix}"

    export E2E_DIR="${e2e_dir}"
    export E2E_ARTIFACTS="${artifacts_directory}"
    export E2E_CLUSTER_NAME="${cluster_name}"
    export E2E_IP_FAMILY="${ip_family}"
    export E2E_BACKEND="${backend}"
    export E2E_DEPLOYMENT_MODEL="${deployment_model}"
    export E2E_EXPORT_METRICS="${export_metrics}"

    [ -d "${artifacts_directory}" ]
    if_error_exit "Directory \"${artifacts_directory}\" does not exist"

    [ -d "${bin_dir}" ]
    if_error_exit "Directory \"${bin_dir}\" does not exist"

    info_message "${cluster_name}"

    create_cluster "${cluster_name}" "${bin_dir}" "${ip_family}" "${artifacts_directory}" "${ci_mode}"
    wait_until_cluster_is_ready "${cluster_name}" "${bin_dir}" "${ci_mode}"

    echo "${cluster_name}" > "${e2e_dir}"/clustername

    if [ "${backend}" != "not-kpng" ] ; then
        install_kpng "${cluster_name}" "${bin_dir}"
    fi
    local result=0
    if ! ${devel_mode} ; then
	run_tests "${e2e_dir}" "${bin_dir}" "false" "${ip_family}" "${backend}" "${include_specific_failed_tests}"
	result=$?
	#need to clean this up
	if [ "${ci_mode}" = false ] ; then
            clean_artifacts "${e2e_dir}" "${bin_dir}" 
	fi
    fi
    return ${result}
}

function delete_kind_clusters {
    ###########################################################################
    # Description:                                                            #
    # delete_kind_clusters                                                    #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: bin_dir                                                         #
    #   arg2: ip_family                                                       #
    #   arg3: backend                                                         #
    #   arg4: suffix                                                          #
    #   arg5: cluser_count                                                    #
    ###########################################################################
    echo "+==================================================================+"
    echo -e "\t\tErasing kind clusters"
    echo "+==================================================================+"

    [ $# -eq 5 ]
    if_error_exit "Erase kind $# Wrong number of arguments to ${FUNCNAME[0]}"

    # setting up variables
    local bin_dir="${1}"
    local ip_family="${2}"
    local backend="${3}"
    local suffix="${4}"
    local cluster_count="${5}"

    add_to_path "${bin_dir}"

    [ "${cluster_count}" -ge "1" ]
    if_error_exit "cluster_count must be larger or equal to one"

    local cluster_name_base="kpng-e2e-${ip_family}-${backend}"

    if [ "${cluster_count}" -eq "1" ] ; then
        local tmp_suffix=${suffix:+"-${suffix}"}
        delete_kind_cluster "${cluster_name_base}${tmp_suffix}" "${bin_dir}"
    else
        for i in $(seq "${cluster_count}"); do
            local tmp_suffix="-${suffix}${i}"
            delete_kind_cluster "${cluster_name_base}${tmp_suffix}" "${bin_dir}"
        done
    fi
}

function print_reports {
    ###########################################################################
    # Description:                                                            #
    # print_reports                                                           #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: ip_family                                                       #
    #   arg2: backend                                                         #
    #   arg3: e2e_directory                                                   #
    #   arg4: suffix                                                          #
    #   arg5: cluster_count                                                   #
    ###########################################################################

    [ $# -eq 5 ]
    if_error_exit "create infra + run $# Wrong number of arguments to ${FUNCNAME[0]}"

    # setting up variables
    local ip_family="${1}"
    local backend="${2}"
    local e2e_directory="${3}"
    local suffix="${4}"
    local cluster_count="${5}"

    echo "+==========================================================================================+"
    echo -e "\t\tTest Report from running test \"-i ${ip_family} -b ${backend}\" on ${cluster_count} clusters."
    echo "+==========================================================================================+"

    local combined_output_file=$(mktemp -q)
    if_error_exit "Could not create temp file, mktemp failed"

    for i in $(seq "${cluster_count}"); do
       local test_directory="${e2e_directory}${suffix}${i}"

       if ! [ -d "${test_directory}" ] ; then
          info_message "directory \"${test_directory}\" not found, skipping"
          continue
       fi

       info_message "Summary report from cluster \"${i}\" in directory: \"${test_directory}\""
       local output_file="${test_directory}/output.log"
       cat "${output_file}" >> "${combined_output_file}"

       sed -nE '/Ran[[:space:]]+[[:digit:]]+[[:space:]]+of[[:space:]]+[[:digit:]]/{N;p}' "${output_file}"
    done

    echo -e "\nOccurrence\tFailure"
    grep "\[FAIL\]" "${combined_output_file}" |
    sort | uniq -c | sort -nr  | sed 's/\,/\n\t\t/g'

    rm -f "${combined_output_file}"
}

function main {
    ###########################################################################
    # Description:                                                            #
    # Starting E2E process                                                    #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################

    [ $# -eq 15 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    # setting up variables
    local ip_family="${1}"
    local backend="${2}"
    local ci_mode="${3}"
    local e2e_dir="${4}"
    local bin_dir="${5}"
    local dockerfile="${6}"
    local suffix="${7}"
    local cluster_count="${8}"
    local erase_clusters="${9}"
    local print_report="${10}"
    local devel_mode="${11}"
    local deployment_model="${12}"
    local run_tests_on_existing_cluster="${13}"
    local export_metrics="${14}"
    local include_specific_failed_tests="${15}"

    [ "${cluster_count}" -ge "1" ]
    if_error_exit "cluster_count must be larger or equal to one"


    e2e_dir=${e2e_dir:="$(pwd)/temp/e2e"}
    bin_dir=${bin_dir:="${e2e_dir}/bin"}

    if ${erase_clusters} ; then
        delete_kind_clusters "${bin_dir}" "${ip_family}" "${backend}" "${suffix}" "${cluster_count}"
        return 0
    fi

    if ${print_report} ; then
        print_reports "${ip_family}" "${backend}" "${e2e_dir}" "-${suffix}" "${cluster_count}"
        return 0
    fi

    echo "+==================================================================+"
    echo -e "\t\tStarting KPNG E2E testing"
    echo "+==================================================================+"

    
    if [ "${run_tests_on_existing_cluster}" = true ] ; then
        run_tests "${e2e_dir}${tmp_suffix}" "${bin_dir}" "false" "${ip_family}" "${backend}" "${include_specific_failed_tests}"
	local result=$?
        #need to clean this up
	if [ "${ci_mode}" = false ] ; then
            clean_artifacts "${e2e_dir}${tmp_suffix}"
	fi
	return ${result}
    fi

    # in ci this should fail
    if [ "${ci_mode}" = true ] ; then
        # REMOVE THIS comment out ON THE REPO WITH A PR WHEN LOCAL TESTS ARE ALL GREEN
        # set -e
        info_message "this tests can't fail now in ci"
    fi
    set_host_network_settings "${ip_family}"

    install_binaries "${bin_dir}" "${E2E_K8S_VERSION}" "${OS}"
    # compile bpf bytecode and bindings so build completes successfully
    if [ "${backend}" == "ebpf" ] ; then
        if [ "${ip_family}" != "ipv4" ] ; then
            echo "ebpf backend only supports ipv4"
            exit 1
        fi
        compile_bpf
    fi

    verify_host_network_settings "${ip_family}"
    prepare_container "${dockerfile}" "${ci_mode}"

    if [ "${cluster_count}" -eq "1" ] ; then
        local tmp_suffix=${suffix:+"-${suffix}"}
        set_e2e_dir "${e2e_dir}${tmp_suffix}" "${bin_dir}"
    else
        for i in $(seq "${cluster_count}"); do
            local tmp_suffix="-${suffix}${i}"
            set_e2e_dir "${e2e_dir}${tmp_suffix}" "${bin_dir}"
        done
    fi

    local result=0
    # preparation completed, time to setup infrastructure and run tests
    if [ "${cluster_count}" -eq "1" ] ; then
        local tmp_suffix=${suffix:+"-${suffix}"}
	create_infrastructure_and_run_tests "${e2e_dir}${tmp_suffix}" "${ip_family}" "${backend}" \
					    "${bin_dir}" "${tmp_suffix}" "${devel_mode}" "${ci_mode}" \
					    "${deployment_model}" "${export_metrics}" "${include_specific_failed_tests}"
	result=$?
    else
        local pids

       echo -e "\n+====================================================================================================+"
       echo -e "\t\tRunning parallel KPNG E2E tests \"-i ${ip_family} -b ${backend}\" in background on ${cluster_count} kind clusters."
       echo -e "+====================================================================================================+"

        for i in $(seq "${cluster_count}"); do
            local tmp_suffix="-${suffix}${i}"
            local output_file="${e2e_dir}${tmp_suffix}/output.log"

            rm -f "${output_file}"
            create_infrastructure_and_run_tests "${e2e_dir}${tmp_suffix}" "${ip_family}" "${backend}" \
						"${bin_dir}" "${tmp_suffix}" "${devel_mode}" "${ci_mode}" \
						"${deployment_model}" "${export_metrics}" "${include_specific_failed_tests}" \
                  &> "${e2e_dir}${tmp_suffix}/output.log" &
            pids[${i}]=$!
        done
        for pid in ${pids[*]}; do # not possible to use quotes here
            wait "${pid}"
	    local tmp=$?
	    if ! [ ${tmp} -eq 0 ] ; then
		result=${tmp}
	    fi
        done
        if ! ${devel_mode} ; then
           print_reports "${ip_family}" "${backend}" "${e2e_dir}" "-${suffix}" "${cluster_count}"
        fi
    fi

    if ${devel_mode} ; then
       echo -e "\n+=====================================================================================+"
       echo -e "\t\tDeveloper mode no test run!"
       echo -e "+=====================================================================================+"
    elif ! ${ci_mode}; then
        delete_kind_clusters "${bin_dir}" "${ip_family}" "${backend}" "${suffix}" "${cluster_count}"
    fi
    
    echo -e "\n+=====================================================================================+"
    echo -e "\t\tResult"
    echo -e "+=====================================================================================+"

    result_message  ${result} "test_e2e test suite for \"-i ${ip_family} -b ${backend}\""
    return ${result}
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
    printf "Usage: %s [-i ip_family] [-b backend]\n" "$0"
    printf "\t-i set ip_family(ipv4/ipv6/dual) name in the e2e test runs.\n"
    printf "\t-b set backend (iptables/nft/ipvs/ebpf/userspacelin/not-kpng/) name in the e2e test runs. \
    \"not-kpng\" is used to be able to validate and compare results\n"
    printf "\t-c flag allows for ci_mode. Please don't run on local systems.\n"
    printf "\t-d devel mode, creates the test env but skip e2e tests. Useful for debugging.\n"
    printf "\t-e erase kind clusters.\n"
    printf "\t-m set the KPNG deployment model, can either be: \n\
            * split-process-per-node [legacy/debug] -> (To run KPNG server + client in separate containers/processes per node)
            * single-process-per-node [default] -> (To run KPNG server + client in a single container/process per node)\n"
    printf "\t-n number of parallel test clusters.\n"
    printf "\t-p flag, only print reports.\n"
    printf "\t-s suffix, will be appended to the E2@ directory and kind cluster name (makes it possible to run parallel tests.\n"
    printf "\t-t Run tests on existing deployment\n"
    printf "\t-B binary directory, specifies the path for the directory where binaries will be installed\n"
    printf "\t-D Dockerfile, specifies the path of the Dockerfile to use\n"
    printf "\t-E set E2E directory, specifies the path for the E2E directory\n"
    printf "\t-I Include failing \"ip_family and backend\" specific test cases\n"
    printf "\t-M Configure kpng to export prometheus metrics\n"
    printf "\t-S Skip the failing \"ip_family and backend\" specific test cases\n"
    printf "\nExample:\n\t %s -i ipv4 -b iptables\n" "${0}\n"
    exit 1 # Exit script after printing help
}
tmp_dir=$(dirname "$0")
base_dir=$(cd "${tmp_dir}" && pwd)
ci_mode=false
devel_mode=false
e2e_dir=""
dockerfile="$(dirname "${base_dir}")/Dockerfile"
bin_dir=""
suffix=""
cluster_count="1"
erase_clusters=false
print_report=false
deployment_model="single-process-per-node"
export_metrics=false
include_specific_failed_tests=true

while getopts "b:cdei:n:mps:t:B:D:E:IMS" flag
do
    case "${flag}" in
        i ) ip_family="${OPTARG}" ;;
        b ) backend="${OPTARG}" ;;
        c ) ci_mode=true ;;
        d ) devel_mode=true ;;
        e ) erase_clusters=true ;;
        n ) cluster_count="${OPTARG}" ;;
        p ) print_report=true ;;
        m ) deployment_model="${OPTARG}" ;;
        s ) suffix="${OPTARG}" ;; 
        t ) run_tests_on_existing_cluster=true ;;
        B ) bin_dir="${OPTARG}" ;;
        D ) dockerfile="${OPTARG}" ;;
        E ) e2e_dir="${OPTARG}" ;;
        I ) include_specific_failed_tests=true ;; 
        M ) export_metrics=true ;; 
        S ) include_specific_failed_tests=false ;; 
        ? ) help ;; #Print help
    esac
done

if  [[ "${cluster_count}" -lt "1" ]]; then
    echo "Cluster count must be larger or equal to 1"
    help
fi
if  [[ "${cluster_count}" -lt "2" ]] && ${print_report}; then
    echo "Cluster count must be larger or equal to 2 when printing reports"
    help
fi

if ! [[ "${backend}" =~ ^(iptables|nft|ipvs|ebpf|userspacelin|not-kpng)$ ]]; then
    echo "user must specify the supported backend"
    help
fi

if [[ -n "${ip_family}" && -n "${backend}" ]]; then
    main "${ip_family}" "${backend}" "${ci_mode}" "${e2e_dir}" "${bin_dir}" "${dockerfile}" \
         "${suffix}" "${cluster_count}" "${erase_clusters}" "${print_report}" "${devel_mode}" \
         "${deployment_model}" "${run_tests_on_existing_cluster}" "${export_metrics}" "${include_specific_failed_tests}"
else
    printf "Both of '-i' and '-b' must be specified.\n"
    help
fi
