#!/bin/bash
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

: ${E2E_GO_VERSION:="1.17.3"}
: ${E2E_K8S_VERSION:="v1.22.2"} 
: ${E2E_TIMEOUT_MINUTES:=100}

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
GINKGO_SKIP_TESTS="Feature|Federation|PerformanceDNS|Disruptive|Serial|LoadBalancer|KubeProxy|GCE|Netpol|NetworkPolicy"
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

function pass_message {
    ###########################################################################
    # Description:                                                            #
    # show [PASSED] in green and a message as the validation passed.          #
    #                                                                         #
    # Arguments:                                                              #
    #   $1 - message to output                                                #
    ###########################################################################
    if ! [ -n "$1" ]; then
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
    #   None                                                                  #
    ###########################################################################
    CONTAINER_FILE="Dockerfile"

    # Running locally it's not necessary to show all info
    QUIET_MODE="--quiet"
    if [ "${ci_mode}" = true ] ; then
        QUIET_MODE=""
        DEV_NULL=""
    fi

    [ -f "${CONTAINER_FILE}" ]
    if_error_exit "cannot find ${CONTAINER_FILE}"

    CMD_BUILD_IMAGE=("${CONTAINER_ENGINE} build ${QUIET_MODE} -t ${KPNG_IMAGE_TAG_NAME} -f Dockerfile .")
    pushd "${0%/*}/.." > /dev/null
        if [ -z "${QUIET_MODE}" ]; then
            ${CMD_BUILD_IMAGE}
        else
            ${CMD_BUILD_IMAGE} &> /dev/null
        
        fi
        if_error_exit "Failed to build kpng, command was: ${CMD_BUILD_IMAGE}"
    popd > /dev/null

    pass_message "Image build and tag ${KPNG_IMAGE_TAG_NAME} is set."
}

function setup_kind {
    ###########################################################################
    # Description:                                                            #
    # setup kind if not available in the system                               #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    if ! kind > /dev/null 2>&1 ; then
        echo -e "\nDownloading kind ..."
        curl -L https://kind.sigs.k8s.io/dl/"${KIND_VERSION}"/kind-"${OS}"-amd64 -o kind
        if_error_exit "cannot download kind"

        sudo chmod +x kind
        sudo mv kind /usr/local/bin/kind
    fi

    pass_message "The kind tool is set."
}

function setup_kubectl {
    ###########################################################################
    # Description:                                                            #
    # setup kubectl if not available in the system                            #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    if ! kubectl > /dev/null 2>&1 ; then
        echo -e "\nDownloading kubectl ..."
        curl -L https://dl.k8s.io/"${E2E_K8S_VERSION}"/bin/"${OS}"/amd64/kubectl -o kubectl
        if_error_exit "cannot download kubectl"

        sudo chmod +x kubectl
        sudo mv kubectl /usr/local/bin/kubectl
    fi

    pass_message "The kubectl tool is set."
}

function setup_ginkgo {
    ###########################################################################
    # Description:                                                            #
    # setup ginkgo and e2e.test                                                #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    if ! [ -f ${E2E_DIR}/ginkgo ] || ! [ -f ${E2E_DIR}/e2e.test ] ; then
        echo -e "\nDownloading ginkgo and e2e.test ..."
        curl -L https://dl.k8s.io/${E2E_K8S_VERSION}/kubernetes-test-${OS}-amd64.tar.gz \
            -o ${E2E_DIR}/kubernetes-test-${OS}-amd64.tar.gz
        if_error_exit "cannot download kubernetes-test package"

        tar xvzf ${E2E_DIR}/kubernetes-test-${OS}-amd64.tar.gz \
            --directory ${E2E_DIR} \
            --strip-components=3 kubernetes/test/bin/ginkgo kubernetes/test/bin/e2e.test &> /dev/null

        rm ${E2E_DIR}/kubernetes-test-${OS}-amd64.tar.gz
        sudo chmod +x "${E2E_DIR}/ginkgo"
        sudo chmod +x "${E2E_DIR}/e2e.test"
    fi

    pass_message "The tools ginko and e2e.test have been set up."
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

function setup_environment {
    ###########################################################################
    # Description:                                                            #
    # Setup Initial Environment                                               #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    mkdir -p "${E2E_DIR}"

    setup_kind
    setup_kubectl
    setup_ginkgo

    if [ "${ci_mode}" = true ] ; then
        apply_ipvx_fixes
    fi
}

function create_cluster {
    ###########################################################################
    # Description:                                                            #
    # Create kind cluster                                                     #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    # Get rid of any old cluster with the same name.
    if kind get clusters | grep -q ${E2E_CLUSTER_NAME} &> /dev/null; then
        kind delete cluster --name ${E2E_CLUSTER_NAME} &> /dev/null
        if_error_exit "cannot delete cluster ${E2E_CLUSTER_NAME}"

        pass_message "Previous cluster ${E2E_CLUSTER_NAME} deleted."
    fi

    KIND_VERBOSE_MODE=""
    if [ "${ci_mode}" = true ] ; then
        KIND_VERBOSE_MODE="-v7"
    fi

    echo -e "\nPreparing to setup ${E2E_CLUSTER_NAME} cluster ..."
    # create cluster
    cat <<EOF | kind create cluster \
        --name ${E2E_CLUSTER_NAME}                     \
        --image "${KINDEST_NODE_IMAGE}":${E2E_K8S_VERSION}    \
        ${VERBOSE_MODE} --wait 1m --retain --config=-
            kind: Cluster
            apiVersion: kind.x-k8s.io/v1alpha4
            networking:
                ipFamily: ${E2E_IP_FAMILY}
            nodes:
            - role: control-plane
            - role: worker
            - role: worker
EOF
    if_error_exit "cannot create kind cluster ${E2E_CLUSTER_NAME}"

    kind get kubeconfig --internal --name ${E2E_CLUSTER_NAME} > "${E2E_ARTIFACTS}/kubeconfig.conf"
    kind get kubeconfig --name ${E2E_CLUSTER_NAME} > "${E2E_ARTIFACTS}/${KUBECONFIG_TESTS}"

    pass_message "Cluster ${E2E_CLUSTER_NAME} is created."
}

function wait_until_cluster_is_ready {
    ###########################################################################
    # Description:                                                            #
    # Wait pods with selector k8s-app=kube-dns be ready and operational       #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    kubectl wait \
        --for=condition=ready \
        pods \
        --namespace="${NAMESPACE}" \
        --selector k8s-app=kube-dns 1> /dev/null

    if [ "${ci_mode}" = true ] ; then
        kubectl get nodes -o wide
        if_error_exit "unable to show nodes"

        kubectl get pods --all-namespaces
        if_error_exit "error getting pods from all namespaces"
    fi

    pass_message "${E2E_CLUSTER_NAME} is operational."
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
    ###########################################################################

    # Get the current config
    original_coredns=$(kubectl get -oyaml -n="${NAMESPACE}" configmap/coredns)
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
    #   None                                                                  #
    ###########################################################################
    # remove kube-proxy
    kubectl delete \
        --namespace "${NAMESPACE}" \
        daemonset.apps/kube-proxy 1> /dev/null
    if_error_exit "cannot delete delete daemonset.apps kube-proxy"
    pass_message "Removed daemonset.apps/kube-proxy."

    # preload kpng image
    # TODO move this to ci:
    # docker load --input kpng-image.tar
    CMD_KIND_LOAD_KPNG_TEST_IMAGE=("kind load docker-image ${KPNG_IMAGE_TAG_NAME} --name ${E2E_CLUSTER_NAME}")
    ${CMD_KIND_LOAD_KPNG_TEST_IMAGE} &> /dev/null
    if_error_exit "error loading image to kind, command was: ${CMD_KIND_LOAD_KPNG_TEST_IMAGE}"
    pass_message "Loaded ${KPNG_IMAGE_TAG_NAME} container image."

    # TODO this should be part of the template                
    kubectl create serviceaccount \
        --namespace "${NAMESPACE}" \
        "${SERVICE_ACCOUNT_NAME}" 1> /dev/null
    if_error_exit "error creating serviceaccount ${SERVICE_ACCOUNT_NAME}"
    pass_message "Created service account ${SERVICE_ACCOUNT_NAME}."

    kubectl create clusterrolebinding \
        "${CLUSTER_ROLE_BINDING_NAME}" \
        --clusterrole="${CLUSTER_ROLE_NAME}" \
        --serviceaccount="${NAMESPACE}":"${SERVICE_ACCOUNT_NAME}" 1> /dev/null
    if_error_exit "error creating clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME}"
    pass_message "Created clusterrolebinding ${CLUSTER_ROLE_BINDING_NAME}."

    kubectl create configmap \
        "${CONFIG_MAP_NAME}" \
        --namespace "${NAMESPACE}" \
        --from-file "${E2E_ARTIFACTS}/kubeconfig.conf" 1> /dev/null
    if_error_exit "error creating configmap ${CONFIG_MAP_NAME}"
    pass_message "Created configmap ${CONFIG_MAP_NAME}."

    # Setting vars for generate the kpng deployment based on template
    export IMAGE=${KPNG_IMAGE_TAG_NAME}
    export PULL=IfNotPresent
    export BACKEND=${E2E_BACKEND}
    envsubst <${0%/*}/kpng-deployment-ds.yaml.tmpl >${E2E_ARTIFACTS}/kpng-deployment-ds.yaml
    if_error_exit "error generating kpng deployment YAML"

    kubectl create -f ${E2E_ARTIFACTS}/kpng-deployment-ds.yaml 1> /dev/null
    if_error_exit "error creating kpng deployment"

    kubectl --namespace="${NAMESPACE}" rollout status daemonset kpng -w --request-timeout=3m 1> /dev/null
    if_error_exit "timeout waiting kpng rollout"

    pass_message "Installation of kpng is done.\n"
}

function run_tests {
    ###########################################################################
    # Description:                                                            #
    # Execute the tests with ginkgo                                           #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    cp "${E2E_ARTIFACTS}/${KUBECONFIG_TESTS}" "${E2E_DIR}/${KUBECONFIG_TESTS}"
    ${E2E_DIR}/ginkgo --nodes="${GINKGO_NUMBER_OF_NODES}" \
        --focus="${GINKGO_FOCUS}" \
        --skip="${GINKGO_SKIP_TESTS}" \
        ${E2E_DIR}/e2e.test \
        -- \
        --kubeconfig="${KUBECONFIG_TESTS}" \
        --provider="${GINKGO_PROVIDER}" \
        --dump-logs-on-failure="${GINKGO_DUMP_LOGS_ON_FAILURE}" \
        --report-dir="${GINKGO_REPORT_DIR}" \
        --disable-log-dump="${GINKGO_DISABLE_LOG_DUMP}"
    if_error_exit "ginkgo: one or more tests failed"
}

function clean_artifacts {
    ###########################################################################
    # Description:                                                            #
    # Clean all artifacts and export kind logs                                #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    kind export \
        logs \
        --name="$(cat $E2E_FILE)" \
        --loglevel="debug" \
        "${E2E_LOGS}"
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
    #   None                                                                  #
    ###########################################################################
    pushd "${0%/*}" > /dev/null
        export E2E_DIR="$(pwd)/temp/e2e"
        export E2E_ARTIFACTS=${E2E_DIR}/artifacts
        mkdir -p ${E2E_DIR}
        mkdir -p ${E2E_ARTIFACTS}
    popd > /dev/null
}

function main {
    ###########################################################################
    # Description:                                                            #
    # Starting E2E process                                                    #
    #                                                                         #
    # Arguments:                                                              #
    #   None                                                                  #
    ###########################################################################
    echo "+==================================================================+"
    echo -e "\t\tStarting KPNG E2E testing"
    echo "+==================================================================+"

    # Detect container engine
    detect_container_engine

    # in ci this should fail
    if [ "${ci_mode}" = true ] ; then 
        # REMOVE THIS comment out ON THE REPO WITH A PR WHEN LOCAL TESTS ARE ALL GREEN
        # set -e
        echo "this tests can't fail now in ci"
    fi

    # setting up variables
    set_e2e_dir
    local ip_family=${1}
    local backend=${2}
    local ci_mode=${3}
    export E2E_CLUSTER_NAME="kpng-e2e-${ip_family}-${backend}"
    export E2E_IP_FAMILY=${ip_family}
    export E2E_BACKEND=${backend}
    mkdir -p ${E2E_ARTIFACTS}

    if [ "${ci_mode}" = true ] ; then
        # store the clustername for other scripts in ci 
        echo "${E2E_CLUSTER_NAME}" 
        echo "${E2E_CLUSTER_NAME}" > ${E2E_DIR}/clustername
    fi
    
    setup_environment
    container_build
    create_cluster
    wait_until_cluster_is_ready
    workaround_coreDNS_for_IPv6_airgapped
    install_kpng

    run_tests

    if [ "${ci_mode}" = false ] ; then
        clean_artifacts
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
    printf "\t-c flag allows for ci_mode. Please don't run on local systems. \n"
    printf "\nExample:\n\t ${0} -i ipv4 -b iptables\n"
    exit 1 # Exit script after printing help
}

ci_mode=false
while getopts "i:b:c" flag
do
    case "${flag}" in
        i ) ip_family="${OPTARG}" ;;
        b ) backend="${OPTARG}" ;;
        c ) ci_mode=true ;;
        ? ) help ;; #Print help
    esac
done

if [[ ! -z "${ip_family}" &&  ! -z "${backend}" ]]; then
   main "${ip_family}" "${backend}" "${ci_mode}"
else
    printf "Both of '-i' and '-b' must be specified.\n"
    help
fi
