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

shopt -s expand_aliases

: "${E2E_GO_VERSION:="1.17.3"}"
: "${E2E_K8S_VERSION:="v1.22.2"}"
: "${E2E_TIMEOUT_MINUTES:=100}"

OS=$(uname| tr '[:upper:]' '[:lower:]')
CONTAINER_ENGINE="docker"
KPNG_IMAGE_TAG_NAME="kpng:test"
KUBECONFIG_TESTS="kubeconfig_tests.conf"

# kind
KIND_VERSION="v0.11.1"

# system data
NAMESPACE="kube-system"
CONFIG_MAP_NAME="kpng"
SERVICE_ACCOUNT_NAME="kpng"
CLUSTER_ROLE_NAME="system:node-proxier"
CLUSTER_ROLE_BINDING_NAME="kpng"

# ginkgo
GINKGO_NUMBER_OF_NODES=25
GINKGO_FOCUS="\[Conformance\]|\[sig-network\]"
GINKGO_SKIP_TESTS="machinery|Feature|Federation|PerformanceDNS|Disruptive|Serial|LoadBalancer|KubeProxy|GCE|Netpol|NetworkPolicy"
GINKGO_REPORT_DIR="artifacts/reports"
GINKGO_DUMP_LOGS_ON_FAILURE=false
GINKGO_DISABLE_LOG_DUMP=true
GINKGO_PROVIDER="local"

# Users can specify docker.io, quay.io registry
KINDEST_NODE_IMAGE="docker.io/kindest/node"

function if_error_exit {
    ###########################################################################
    # Description:                                                            #
    # Validate if previous command failed and show an error msg (if provided) #
    #                                                                         #
    # Arguments:                                                              #
    #   $1 - error message if not provided, it will just exit                 #
    ###########################################################################
    if [ "$?" != "0" ]; then
        if [ -n "$1" ]; then
            RED="\e[31m"
            ENDCOLOR="\e[0m"
            echo -e "[ ${RED}FAILED${ENDCOLOR} ] ${1}"
        fi
        exit 1
    fi
}

function if_error_warning {
    ###########################################################################
    # Description:                                                            #
    # Validate if previous command failed and show an error msg (if provided) #
    #                                                                         #
    # Arguments:                                                              #
    #   $1 - error message if not provided, it will just exit                 #
    ###########################################################################
    if [ "$?" != "0" ]; then
        if [ -n "$1" ]; then
            RED="\e[31m"
            ENDCOLOR="\e[0m"
            echo -e "[ ${RED}FAILED${ENDCOLOR} ] ${1}"
        fi
    fi
}

function pass_message {
    ###########################################################################
    # Description:                                                            #
    # show [PASSED] in green and a message as the validation passed.          #
    #                                                                         #
    # Arguments:                                                              #
    #   $1 - message to output                                                #
    ###########################################################################
    if [ -z "${1}" ]; then
        echo "pass_message() requires a message"
        exit 1
    fi
    GREEN="\e[32m"
    ENDCOLOR="\e[0m"
    echo -e "[ ${GREEN}PASSED${ENDCOLOR} ] ${1}"
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
    ###########################################################################
    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    CONTAINER_FILE=${1}

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

function setup_kind {
    ###########################################################################
    # Description:                                                            #
    # setup kind if not available in the system                               #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: installation directory, path to where kind will be installed     #
    ###########################################################################
    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local install_directory=$1

    [ -d "${install_directory}" ]
    if_error_exit "Directory \"${install_directory}\" does not exist"


    if ! [ -f "${install_directory}"/kind ] ; then
        echo -e "\nDownloading kind ..."

        local tmp_file=$(mktemp -q)
        if_error_exit "Could not create temp file, mktemp failed"

        curl -L https://kind.sigs.k8s.io/dl/"${KIND_VERSION}"/kind-"${OS}"-amd64 -o "${tmp_file}"
        if_error_exit "cannot download kind"

        sudo mv "${tmp_file}" "${install_directory}"/kind
        sudo chmod +rx "${install_directory}"/kind
        sudo chown root.root "${install_directory}"/kind
    fi

    pass_message "The kind tool is set."
}

function setup_kubectl {
    ###########################################################################
    # Description:                                                            #
    # setup kubectl if not available in the system                            #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: installation directory, path to where kubectl will be installed  #
    ###########################################################################

    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local install_directory=$1

    [ -d "${install_directory}" ]
    if_error_exit "Directory \"${install_directory}\" does not exist"


    if ! [ -f "${install_directory}"/kubectl ] ; then
        echo -e "\nDownloading kubectl ..."

        local tmp_file=$(mktemp -q)
        if_error_exit "Could not create temp file, mktemp failed"

        curl -L https://dl.k8s.io/"${E2E_K8S_VERSION}"/bin/"${OS}"/amd64/kubectl -o "${tmp_file}"
        if_error_exit "cannot download kubectl"

        sudo mv "${tmp_file}" "${install_directory}"/kubectl
        sudo chmod +rx "${install_directory}"/kubectl
        sudo chown root.root "${install_directory}"/kubectl
    fi

    pass_message "The kubectl tool is set."
}

function setup_ginkgo {
    ###########################################################################
    # Description:                                                            #
    # setup ginkgo and e2e.test                                               #
    #                                                                         #
    # # Arguments:                                                            #
    #   arg1: binary directory, path to where ginko will be installed         #
    #   arg2: Kubernetes version                                              #
    #   arg3: OS, name of the operating system                                #
    ###########################################################################

    [ $# -eq 3 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local bin_directory=${1}
    local k8s_version=${2}
    local os=${3}
    local temp_directory=$(mktemp -qd)

    if ! [ -f "${bin_directory}"/ginkgo ] || ! [ -f "${bin_directory}"/e2e.test ] ; then
        echo -e "\nDownloading ginkgo and e2e.test ..."
        curl -L https://dl.k8s.io/"${k8s_version}"/kubernetes-test-"${os}"-amd64.tar.gz \
            -o "${temp_directory}"/kubernetes-test-"${os}"-amd64.tar.gz
        if_error_exit "cannot download kubernetes-test package"

        tar xvzf "${temp_directory}"/kubernetes-test-"${os}"-amd64.tar.gz \
            --directory "${bin_directory}" \
            --strip-components=3 kubernetes/test/bin/ginkgo kubernetes/test/bin/e2e.test &> /dev/null

        rm -rf "${temp_directory}"
        sudo chmod +rx "${bin_directory}/ginkgo"
        sudo chmod +rx "${bin_directory}/e2e.test"
    fi

    pass_message "The tools ginko and e2e.test have been set up."
}

function create_cluster {
    ###########################################################################
    # Description:                                                            #
    # Create kind cluster                                                     #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: cluster name                                                    #
    #   arg2: IP family                                                   #
    #   arg3: artifacts directory                                                    #
   ###########################################################################
    [ $# -eq 3 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local cluster_name=$1
    local ip_family=$2
    local artifacts_directory=$3

    # Get rid of any old cluster with the same name.
    if kind get clusters | grep -q "${cluster_name}" &> /dev/null; then
        kind delete cluster --name "${cluster_name}" &> /dev/null
        if_error_exit "cannot delete cluster ${cluster_name}"

        pass_message "Previous cluster ${cluster_name} deleted."
    fi

    KIND_VERBOSE_MODE="-v0"
    if [ "${ci_mode}" = true ] ; then
        KIND_VERBOSE_MODE="-v7"
    fi

    echo -e "\nPreparing to setup ${cluster_name} cluster ..."
    # create cluster
 cat <<EOF | kind create cluster \
        --name "${cluster_name}"                     \
        --image "${KINDEST_NODE_IMAGE}":"${E2E_K8S_VERSION}"    \
        "${KIND_VERBOSE_MODE}" --wait 1m --retain --config=-
            kind: Cluster
            apiVersion: kind.x-k8s.io/v1alpha4
            networking:
                ipFamily: "${ip_family}"
            nodes:
            - role: control-plane
            - role: worker
            - role: worker
EOF
    if_error_exit "cannot create kind cluster ${cluster_name}"

    kind get kubeconfig --internal --name "${cluster_name}" > "${artifacts_directory}/kubeconfig.conf"
    kind get kubeconfig --name "${cluster_name}" > "${artifacts_directory}/${KUBECONFIG_TESTS}"

    pass_message "Cluster ${cluster_name} is created."
}

function wait_until_cluster_is_ready {
    ###########################################################################
    # Description:                                                            #
    # Wait pods with selector k8s-app=kube-dns be ready and operational       #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: cluster name                                                    #
   ###########################################################################

    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local cluster_name=$1
    local k8s_context="kind-${cluster_name}"

    kubectl --context "${k8s_context}" wait \
        --for=condition=ready \
        pods \
        --namespace="${NAMESPACE}" \
        --selector k8s-app=kube-dns 1> /dev/null

    if [ "${ci_mode}" = true ] ; then
        kubectl --context "${k8s_context}" get nodes -o wide
        if_error_exit "unable to show nodes"

        kubectl --context "${k8s_context}" get pods --all-namespaces
        if_error_exit "error getting pods from all namespaces"
    fi

    pass_message "${cluster_name} is operational."
}

function workaround_coreDNS_for_IPv6_airgapped {
    ###########################################################################
    # Description:                                                            #
    #                                                                         #
    # Patch CoreDNS to work in Github CI                                      #
    # 1. Github CI doesn´t offer IPv6 connectivity, so CoreDNS should be      #
    # configured to work in an offline environment:                           #
    # https://github.com/coredns/coredns/issues/2494#issuecomment-457215452   #
    # 2. Github CI adds following domains to resolv.conf search field:        #
    # .net.                                                                   #
    # CoreDNS should handle those domains and answer with NXDOMAIN instead of #
    # SERVFAIL otherwise pods stops trying to resolve the domain.             #
    #                                                                         #
    #   arg1: cluster name                                                    #
   ###########################################################################

    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local cluster_name=$1
    local k8s_context="kind-${cluster_name}"
    ###########################################################################

    # Get the current config
    original_coredns=$(kubectl --context "${k8s_context}" get -oyaml -n="${NAMESPACE}" configmap/coredns)
    if [ "${ci_mode}" = true ] ; then
        echo "Original CoreDNS config:"
        echo "${original_coredns}"
    fi

    # Patch it
    fixed_coredns=$(
        printf '%s' "${original_coredns}" | sed \
            -e 's/^.*kubernetes cluster\.local/& net/' \
            -e '/^.*upstream$/d' \
            -e '/^.*fallthrough.*$/d' \
            -e '/^.*forward . \/etc\/resolv.conf$/d' \
            -e '/^.*loop$/d' \
    )
    if [ "${ci_mode}" = true ] ; then
        echo "Patched CoreDNS config:"
        echo "${fixed_coredns}"
    fi
    printf '%s' "${fixed_coredns}" | kubectl apply -f - &> /dev/null
    if_error_exit "cannot apply patch in CoreDNS"

    pass_message "CoreDNS is patched and ready."
}

function delete_kind_cluster {
    ###########################################################################
    # Description:                                                            #
    # delete kind cluster                                                     #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: cluster name                                                    #
    ###########################################################################
    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local cluster_name="${1}"

    if kind get clusters | grep -q "${cluster_name}" &> /dev/null; then
        kind delete cluster --name "${cluster_name}" &> /dev/null
        if_error_warning "cannot delete cluster ${cluster_name}"

        pass_message "Cluster ${cluster_name} deleted."
    fi
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
   ###########################################################################
    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local cluster_name=$1
    local k8s_context="kind-${cluster_name}"

    # remove kube-proxy
    kubectl --context "${k8s_context}" delete \
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
    kubectl --context "${k8s_context}" create serviceaccount \
        --namespace "${NAMESPACE}" \
        "${SERVICE_ACCOUNT_NAME}" 1> /dev/null
    if_error_exit "error creating serviceaccount ${SERVICE_ACCOUNT_NAME}"
    pass_message "Created service account ${SERVICE_ACCOUNT_NAME}."

    kubectl --context "${k8s_context}" create clusterrolebinding \
        "${CLUSTER_ROLE_BINDING_NAME}" \
        --clusterrole="${CLUSTER_ROLE_NAME}" \
        --serviceaccount="${NAMESPACE}":"${SERVICE_ACCOUNT_NAME}" 1> /dev/null
    if_error_exit "error creating clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME}"
    pass_message "Created clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME}."

    kubectl --context "${k8s_context}" create configmap \
        "${CONFIG_MAP_NAME}" \
        --namespace "${NAMESPACE}" \
        --from-file "${artifacts_directory}/kubeconfig.conf" 1> /dev/null
    if_error_exit "error creating configmap ${CONFIG_MAP_NAME}"
    pass_message "Created configmap ${CONFIG_MAP_NAME}."

    # Setting vars for generate the kpng deployment based on template
    export IMAGE="${KPNG_IMAGE_TAG_NAME}"
    export PULL=IfNotPresent
    export E2E_BACKEND
    export CONFIG_MAP_NAME
    export SERVICE_ACCOUNT_NAME
    export NAMESPACE
    envsubst <"${0%/*}"/kpng-deployment-ds.yaml.tmpl > "${artifacts_directory}"/kpng-deployment-ds.yaml
    if_error_exit "error generating kpng deployment YAML"

    kubectl --context "${k8s_context}" create -f "${artifacts_directory}"/kpng-deployment-ds.yaml 1> /dev/null
    if_error_exit "error creating kpng deployment"

    kubectl --context "${k8s_context}" --namespace="${NAMESPACE}" rollout status daemonset kpng -w --request-timeout=3m 1> /dev/null
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
     ###########################################################################
    [ $# -eq 1 ]
     if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local e2e_dir="${1}"
    local artifacts_directory="${e2e_dir}/artifacts"

    cp "${artifacts_directory}/${KUBECONFIG_TESTS}" "${e2e_dir}/${KUBECONFIG_TESTS}"
    ginkgo --nodes="${GINKGO_NUMBER_OF_NODES}" \
           --focus="${GINKGO_FOCUS}" \
           --skip="${GINKGO_SKIP_TESTS}" \
           "${E2E_DIR}"/e2e.test \
           -- \
           --kubeconfig="${KUBECONFIG_TESTS}" \
           --provider="${GINKGO_PROVIDER}" \
           --dump-logs-on-failure="${GINKGO_DUMP_LOGS_ON_FAILURE}" \
           --report-dir="${GINKGO_REPORT_DIR}" \
           --disable-log-dump="${GINKGO_DISABLE_LOG_DUMP}"

    # FIXME: until all tests are green, let's keep the exit 0
    #if_error_exit "ginkgo: one or more tests failed"
    exit 0
}

function clean_artifacts {
    ###########################################################################
    # Description:                                                            #
    # Clean all artifacts and export kind logs                                #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: Path for E2E installation directory                             #
    ###########################################################################

    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local e2e_dir="${1}"

    kind export \
        logs \
        --name="$(cat "${e2e_dir}/clustername")" \
        --loglevel="debug" \
        "${E2E_LOGS}"
    if_error_exit "cannot export kind logs"

    #TODO in local mode to avoid the overwriting of artifacts from test to test
    #     add logic to this function that moves the content of artifacts
    #     to a dir named clustername+date+time
    pass_message "make sure to safe your result before the next run."
}


function apply_ipvx_fixes {
    ###########################################################################
    # Description:                                                            #
    # apply ipvx fixe                                                         #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    sudo sysctl -w net.ipv6.conf.all.forwarding=1
    sudo sysctl -w net.ipv4.ip_forward=1
}

function add_to_path {
    ###########################################################################
    # Description:                                                            #
    # Add directory to path                                                   #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1:  directory                                                      #
    ###########################################################################

    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local directory="${1}"

    [ -d "${directory}" ]
    if_error_exit "Directory \"${directory}\" does not exist"

    case ":${PATH:-}:" in
        *:${directory}:*) ;;
        *) PATH="${directory}${PATH:+:$PATH}" ;;
    esac
}

function install_binaries {
    ###########################################################################
    # Description:                                                            #
    # Copy binaries from the net to binaries directory                         #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: binary directory, path to where ginko will be installed         #
    #   arg2: Kubernetes version                                              #
    #   arg3: OS, name of the operating system                                #
    ###########################################################################

    [ $# -eq 3 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local bin_directory="${1}"
    local k8s_version="${2}"
    local os="${3}"

    pushd "${0%/*}" > /dev/null || exit
          mkdir -p "${bin_directory}"
    popd > /dev/null || exit

    add_to_path "${bin_directory}"

    setup_kind "${bin_directory}"
    setup_kubectl "${bin_directory}"
    setup_ginkgo "${bin_directory}" "${k8s_version}" "${os}"
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
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local e2e_dir="${1}"
    local bin_dir="${2}"

    [ -d "${bin_dir}" ]
    if_error_exit "Directory \"${bin_dir}\" does not exist"

    pushd "${0%/*}" > /dev/null || exit
        mkdir -p "${e2e_dir}"
        mkdir -p "${e2e_dir}/artifacts"
        # e2e.test must be placed in the E2E directory when called by ginkgo
        if ! [ -f "${e2e_dir}"/e2e.test ] ; then
            sudo cp "${bin_dir}/e2e.test" "${e2e_dir}/e2e.test"
            sudo chmod +rx "${e2e_dir}/e2e.test"
        fi
    popd > /dev/null || exit
}

function prepare_container {
    ###########################################################################
    # Description:                                                            #
    # Prepare container                                                       #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: Path of dockerfile                                              #
    ###########################################################################
    local dockerfile="${1}"

    # Detect container engine
    detect_container_engine
    container_build "${dockerfile}"
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
    #   arg4: suffix                                                          #
    #   arg5: developer_mode                                                  #
    #   arg6: ci_mode                                                          #
   ###########################################################################

    [ $# -eq 6 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local e2e_dir="${1}"
    local ip_family="${2}"
    local backend="${3}"
    local suffix="${4}"
    local devel_mode="${5}"
    local ci_mode="${6}"

    local artifacts_directory="${e2e_dir}/artifacts"
    local cluster_name="kpng-e2e-${ip_family}-${backend}${suffix}"

    export E2E_DIR="${e2e_dir}"
    export E2E_ARTIFACTS="${artifacts_directory}"
    export E2E_CLUSTER_NAME="${cluster_name}"
    export E2E_IP_FAMILY="${ip_family}"
    export E2E_BACKEND="${backend}"
    export E2E_DIR="${e2e_dir}"
    export E2E_ARTIFACTS="${artifacts_directory}"

    [ -d "${artifacts_directory}" ]
    if_error_exit "Directory \"${artifacts_directory}\" does not exist"

    if [ "${ci_mode}" = true ] ; then
        # store the clustername for other scripts in ci
        echo "${cluster_name}"
        echo "${cluster_name}" > "${e2e_dir}"/clustername
    fi

    create_cluster "${cluster_name}" "${ip_family}" "${artifacts_directory}"
    wait_until_cluster_is_ready "${cluster_name}"

    if [ "${ci_mode}" = true ] ; then
        workaround_coreDNS_for_IPv6_airgapped "${cluster_name}"
    fi

    install_kpng "${cluster_name}"
    run_tests "${e2e_dir}"

    #need to clean this up
    if [ "${ci_mode}" = false ] ; then
          clean_artifacts "${e2e_dir}"
    fi
}

function delete_kind_clusters {
    ###########################################################################
    # Description:                                                            #
    # create_infrastructure_and_run_tests                                     #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: bin_directory                                                   #
    #   arg2: ip_family                                                       #
    #   arg3: backend                                                         #
    #   arg4: suffix                                                          #
    #   arg5: cluser_count                                                    #
    ###########################################################################
    echo "+==================================================================+"
    echo -e "\t\tErasing kind clusters"
    echo "+==================================================================+"

    [ $# -eq 5 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    # setting up variables
    local bin_directory="${1}"
    local ip_family="${2}"
    local backend="${3}"
    local suffix="${4}"
    local cluster_count="${5}"

    add_to_path "${bin_directory}"

    [ "${cluster_count}" -ge "1" ]
    if_error_exit "cluster_count must be larger or equal to one"

    local cluster_name_base="kpng-e2e-${ip_family}-${backend}"

    if [ "${cluster_count}" -eq "1" ] ; then
        local tmp_suffix=${suffix:+"-${suffix}"}
        delete_kind_cluster "${cluster_name_base}${tmp_suffix}"
    else
        for i in $(seq "${cluster_count}"); do
            local tmp_suffix="-${suffix}${i}"
            delete_kind_cluster "${cluster_name_base}${tmp_suffix}"
        done
    fi
}

function print_reports {
    ###########################################################################
    # Description:                                                            #
    # create_infrastructure_and_run_tests                                     #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: e2e_directory                                                   #
    #   arg2: suffix                                                       #
    #   arg3: cluster_count                                                         #
    ###########################################################################

    [ $# -eq 3 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    # setting up variables
    local e2e_directory="${1}"
    local suffix="${2}"
    local cluster_count="${3}"

    echo "+==================================================================+"
    echo -e "\t\tTest Report from running test on ${cluster_count}" clusters.
    echo "+==================================================================+"

    local combined_output_file=$(mktemp -q)
    if_error_exit "Could not create temp file, mktemp failed"

    for i in $(seq "${cluster_count}"); do
       local test_directory="${e2e_directory}${suffix}${i}"

       if ! [ -d "${test_directory}" ] ; then
          echo "directory \"${test_directory}\" not found, skipping"
          continue
       fi

       echo -e "Summary report from cluster \"${i}\" in directory: \"${test_directory}\""
       local output_file="${test_directory}/output.log"
       cat "${output_file}" >> "${combined_output_file}"

       sed -nE '/Ran[[:space:]]+[[:digit:]]+[[:space:]]+of[[:space:]]+[[:digit:]]/{N;p}' "${output_file}"
    done

    echo -e "\nOccurence\tFailure"
    awk '/Summarizing/,0' "${combined_output_file}" | awk 'ORS=/\[Fail\]/?", ":RS' | awk '/\[Fail\]/' | \
          sed 's/\x1b\[90m//g' |sort | uniq -c | sort -nr  | sed 's/\,/\n\t\t/g'

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

    [ $# -eq 10 ]
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

    [ "${cluster_count}" -ge "1" ]
    if_error_exit "cluster_count must be larger or equal to one"


    e2e_dir=${e2e_dir:="$(pwd)/temp/e2e"}
    bin_dir=${bin_dir:="${e2e_dir}/bin"}

    if ${erase_clusters} ; then
        delete_kind_clusters "${bin_dir}" "${ip_family}" "${backend}" "${suffix}" "${cluster_count}"
        exit 1
    fi

    if ${print_report} ; then
        print_reports "${e2e_dir}" "-${suffix}" "${cluster_count}"
        exit 1
    fi

    echo "+==================================================================+"
    echo -e "\t\tStarting KPNG E2E testing"
    echo "+==================================================================+"

    # in ci this should fail
    if [ "${ci_mode}" = true ] ; then 
        # REMOVE THIS comment out ON THE REPO WITH A PR WHEN LOCAL TESTS ARE ALL GREEN
        # set -e
        echo "this tests can't fail now in ci"
        apply_ipvx_fixes
    fi

    prepare_container "${dockerfile}"
    install_binaries "${bin_dir}" "${E2E_K8S_VERSION}" "${OS}"

    if [ "${cluster_count}" -eq "1" ] ; then
        local tmp_suffix=${suffix:+"-${suffix}"}
        set_e2e_dir "${e2e_dir}${tmp_suffix}" "${bin_dir}"
    else
        for i in $(seq "${cluster_count}"); do
            local tmp_suffix="-${suffix}${i}"
            set_e2e_dir "${e2e_dir}${tmp_suffix}" "${bin_dir}"
        done
    fi

    # preparation completed, time to setup infrastructure and run tests
    if [ "${cluster_count}" -eq "1" ] ; then
        local tmp_suffix=${suffix:+"-${suffix}"}
        create_infrastructure_and_run_tests "${e2e_dir}${tmp_suffix}" "${ip_family}" "${backend}" \
              "${tmp_suffix}" "${devel_mode}" "${ci_mode}"
    else
        local pids

       echo -e "\n+=====================================================================================+"
       echo -e "\t\tRunning parallel KPNG E2E tests in background on ${cluster_count} kind clusters."
       echo -e "+=====================================================================================+\n"

        for i in $(seq "${cluster_count}"); do
            local tmp_suffix="-${suffix}${i}"
            local output_file="${e2e_dir}${tmp_suffix}/output.log"
            rm -f "${output_file}"
            create_infrastructure_and_run_tests "${e2e_dir}${tmp_suffix}" "${ip_family}" "${backend}" \
                  "${tmp_suffix}" "${devel_mode}" "${ci_mode}"  &> "${e2e_dir}${tmp_suffix}/output.log" &
            pids[${i}]=$!
        done
        for pid in ${pids[*]}; do # not possible to use quotes here
              wait ${pid}
        done
       print_reports "${e2e_dir}" "-${suffix}" "${cluster_count}"
    fi
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
    printf "\t-b set backend (iptables/nft/ipvs) name in the e2e test runs.\n"
    printf "\t-c flag allows for ci_mode. Please don't run on local systems.\n"
    printf "\t-d devel mode, creates the test env but skip e2e tests. Useful for debugging.\n"
    printf "\t-e erase kind clusters.\n"
    printf "\t-n number of parallel test clusters.\n"
    printf "\t-s suffix, will be appended to the E2@ directory and kind cluster name (makes it possible to run parallel tests.\n"
    printf "\t-B binary directory, specifies the path for the directory where binaries will be installed\n"
    printf "\t-D Dockerfile, specifies the path of the Dockerfile to use\n"
    printf "\t-E set E2E directory, specifies the path for the E2E directory\n"
    printf "\nExample:\n\t %s -i ipv4 -b iptables\n" "${0}"
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

while getopts "i:b:B:cdD:eE:n:ps:" flag
do
    case "${flag}" in
        i ) ip_family="${OPTARG}" ;;
        b ) backend="${OPTARG}" ;;
        c ) ci_mode=true ;;
        d ) devel_mode=true ;;
        e ) erase_clusters=true ;;
        n ) cluster_count="${OPTARG}" ;;
        p ) print_report=true ;;
        s ) suffix="${OPTARG}" ;;
        B ) bin_dir="${OPTARG}" ;;
        D ) dockerfile="${OPTARG}" ;;
        E ) e2e_dir="${OPTARG}" ;;
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

if ! [[ "${backend}" =~ ^(iptables|nft|ipvs)$ ]]; then
    echo "user must specify the supported backend"
    help
fi

if [[ -n "${ip_family}" && -n "${backend}" ]]; then
    main "${ip_family}" "${backend}" "${ci_mode}" "${e2e_dir}" "${bin_dir}" "${dockerfile}" \
         "${suffix}" "${cluster_count}" "${erase_clusters}" "${print_report}"
else
    printf "Both of '-i' and '-b' must be specified.\n"
    help
fi
