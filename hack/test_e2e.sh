#!/bin/bash

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
            echo -e "[${RED}FAILED${ENDCOLOR}] ${1}"
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
    echo -e "[${GREEN}PASSED${ENDCOLOR}] ${1}"
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

    echo "* Detecting container engine..."
    # If docker is not available, let's check if podman exists
    ${CONTAINER_ENGINE} &> /dev/null
    if [ "$?" != "0" ]; then
        CONTAINER_ENGINE="podman"
        ${CONTAINER_ENGINE} --help &> /dev/null
        if_error_exit "the e2e tests currently support docker and podman as the container engine. Please install either of them"
    fi
    pass_message "Detected Container Engine: ${CONTAINER_ENGINE}"
}

function line {
    echo "+============================================================================+"
}

function docker_build {
    line
    echo "   Resolving kpng docker image"
    line

    CMD_BUILD_IMAGE=("${CONTAINER_ENGINE} build -t kpng:test -f Dockerfile .")
    pushd "${0%/*}/.." > /dev/null
        ${CMD_BUILD_IMAGE}
        if_error_exit "Failed to build kpng, command was: ${CMD_BUILD_IMAGE}"

        echo "docker image build."
    popd > /dev/null

    echo -e "Let's move on.\n"
}

function setup_kind {
    line
    echo "   Resolving kind bin"
    line

    if ! kind > /dev/null 2>&1 ; then
        echo "kind not found"
        if [ "${ci_mode}" = true ] ; then
            echo "pulling binary ..."
            curl -L https://kind.sigs.k8s.io/dl/v0.11.1/kind-${OS}-amd64 -o kind
            sudo chmod +x kind
            sudo mv kind /usr/local/bin/kind
            echo "kind was set up."
        else
            line
            echo "please get kind and add it to PATH"
            echo "https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
            exit 1
        fi
    else
        echo "kind bin found."
    fi

    echo -e "Let's move on.\n"
}

function setup_kubectl {
    line
    echo "   Resolving kubectl bin"
    line

    if ! kubectl > /dev/null 2>&1 ; then
        echo "kubectl not found"
        if [ "${ci_mode}" = true ] ; then
            echo "pulling binary ..."
            curl -L https://dl.k8s.io/${K8S_VERSION}/bin/${OS}/amd64/kubectl -o kubectl
            sudo chmod +x kubectl
            sudo mv kubectl /usr/local/bin/kubectl
            echo "kubectl was set up."
        else
            line
            echo "please get kubectl and add it to PATH"
            echo "https://kubernetes.io/docs/tasks/tools/#kubectl"
            exit 1
        fi
    else
        echo "kubectl bin found."
    fi

    echo -e "Let's move on.\n"
}

function setup_ginkgo {
    line
    echo "   Resolving e2e test bins"
    line

    if ! [ -f ${E2E_DIR}/ginkgo ] || ! [ -f ${E2E_DIR}/e2e.test ] ; then
        echo "ginko and/or e2e.test, pulling binaries ..."
        curl -L https://dl.k8s.io/${E2E_K8S_VERSION}/kubernetes-test-${OS}-amd64.tar.gz \
            -o ${E2E_DIR}/kubernetes-test-${OS}-amd64.tar.gz
        tar xvzf ${E2E_DIR}/kubernetes-test-${OS}-amd64.tar.gz \
            --directory ${E2E_DIR} \
            --strip-components=3 kubernetes/test/bin/ginkgo kubernetes/test/bin/e2e.test
        rm ${E2E_DIR}/kubernetes-test-${OS}-amd64.tar.gz
        sudo chmod +x "${E2E_DIR}/ginkgo"
        sudo chmod +x "${E2E_DIR}/e2e.test"
        echo "ginko and e2e.test have been set up."
    else
        echo "ginko and e2e.test found."
    fi

    echo -e "Let's move on.\n"
}

function apply_ipvx_fixes {
    if ci_mode = true ; then
        sudo sysctl -w net.ipv6.conf.all.forwarding=1
        sudo sysctl -w net.ipv4.ip_forward=1
    fi
}

function setup_environment {
    mkdir -p "${E2E_DIR}"

    setup_kind
    setup_kubectl
    setup_ginkgo

    apply_ipvx_fixes
}

function create_cluster {
    line
    echo "  Building the cluster '${E2E_CLUSTER_NAME}'"
    line

    # Get rid of any old cluster with the same name.
    if kind get clusters | grep -q ${E2E_CLUSTER_NAME}; then
        kind delete cluster --name ${E2E_CLUSTER_NAME}
    fi
    # create cluster

    cat <<EOF | kind create cluster \
        --name ${E2E_CLUSTER_NAME}                     \
        --image kindest/node:${E2E_K8S_VERSION}    \
        -v7 --wait 1m --retain --config=-
            kind: Cluster
            apiVersion: kind.x-k8s.io/v1alpha4
            networking:
                ipFamily: ${E2E_IP_FAMILY}
            nodes:
            - role: control-plane
            - role: worker
            - role: worker
EOF
    kind get kubeconfig --internal --name ${E2E_CLUSTER_NAME} > "${E2E_ARTIFACTS}/kubeconfig.conf"
    kind get kubeconfig --name ${E2E_CLUSTER_NAME} > "${E2E_ARTIFACTS}/kubeconfig_tests.conf"
    echo "cluster is up"
    echo -e "Let's move on.\n"
}

function wait_until_cluster_is_ready {
    line
    echo "  Waiting for the cluster '${E2E_CLUSTER_NAME}' to be ready"
    line

    kubectl wait --for=condition=ready pods --namespace=kube-system -l k8s-app=kube-dns
    kubectl get nodes -o wide
    kubectl get pods -A

    echo -e "Let's move on.\n"
}

function workaround_coreDNS_for_IPv6_airgapped {
    # Patch CoreDNS to work in Github CI
    # 1. Github CI doesn´t offer IPv6 connectivity, so CoreDNS should be configured
    # to work in an offline environment:
    # https://github.com/coredns/coredns/issues/2494#issuecomment-457215452
    # 2. Github CI adds following domains to resolv.conf search field:
    # .net.
    # CoreDNS should handle those domains and answer with NXDOMAIN instead of SERVFAIL
    # otherwise pods stops trying to resolve the domain.

    # Get the current config

    line
    echo "  Let's patch CoreDNS"
    line

    original_coredns=$(kubectl get -oyaml -n=kube-system configmap/coredns)
    echo "Original CoreDNS config:"
    echo "${original_coredns}"

    # Patch it
    fixed_coredns=$(
        printf '%s' "${original_coredns}" | sed \
            -e 's/^.*kubernetes cluster\.local/& net/' \
            -e '/^.*upstream$/d' \
            -e '/^.*fallthrough.*$/d' \
            -e '/^.*forward . \/etc\/resolv.conf$/d' \
            -e '/^.*loop$/d' \
    )
    echo "Patched CoreDNS config:"
    echo "${fixed_coredns}"
    printf '%s' "${fixed_coredns}" | kubectl apply -f -

    echo -e "Let's move on.\n"
}

function install_kpng {
    line
    echo "   Installing kpng on the cluster ${E2E_CLUSTER_NAME}"
    line
    
    # remove kube-proxy
    echo "removing kube-proxy"
    kubectl -n kube-system delete daemonset.apps/kube-proxy

    # preload kpng image
    # TODO move this to ci:
    # docker load --input kpng-image.tar
    echo "loading kpng:test docker image"
    kind load docker-image kpng:test --name ${E2E_CLUSTER_NAME}

    # TODO this should be part of the template                
    kubectl -n kube-system create sa kpng
    kubectl create clusterrolebinding kpng --clusterrole=system:node-proxier --serviceaccount=kube-system:kpng
    kubectl -n kube-system create cm kpng --from-file "${E2E_ARTIFACTS}/kubeconfig.conf"
    echo "Applying template"
    export IMAGE=kpng:test
    export PULL=IfNotPresent
    export BACKEND=${E2E_BACKEND}
    envsubst <${0%/*}/kpng-deployment-ds.yaml.tmpl >${E2E_ARTIFACTS}/kpng-deployment-ds.yaml
    echo "deploying kpng daemonset"
    kubectl create -f ${E2E_ARTIFACTS}/kpng-deployment-ds.yaml
    echo "any second now ..."
    kubectl --namespace=kube-system rollout status daemonset kpng -w --timeout=3m
    echo "installation of kpng is done."

    echo -e "Let's move on.\n"
}

function run_tests {
    cp ${E2E_ARTIFACTS}/kubeconfig_tests.conf ${E2E_DIR}/kubeconfig_tests.conf 
    ${E2E_DIR}/ginkgo --nodes=25 \
        --focus="\[Conformance\]|\[sig-network\]" \
        --skip="Feature|Federation|PerformanceDNS|Disruptive|Serial|LoadBalancer|KubeProxy|GCE|Netpol|NetworkPolicy" \
        ${E2E_DIR}/e2e.test \
        -- \
        --kubeconfig=kubeconfig_tests.conf \
        --provider=local \
        --dump-logs-on-failure=false \
        --report-dir=artifacts/reports \
        --disable-log-dump=true
}

function clean_artifacts {
    kind export logs --name="$(cat $E2E_FILE)" --loglevel="debug" "${E2E_LOGS}"

    #TODO in local mode to avoid the overwriting of artifacts from test to test
    #     add logic to this function that moves the content of artifacts
    #     to a dir named clustername+date+time
    echo "make sure to safe your result before the next run."
}

function set_e2e_dir {
    pushd "${0%/*}" > /dev/null
        export E2E_DIR="$(pwd)/temp/e2e"
        export E2E_ARTIFACTS=${E2E_DIR}/artifacts
        mkdir -p ${E2E_DIR}
        mkdir -p ${E2E_ARTIFACTS}
    popd > /dev/null
}

function main {

    echo "* Starting KPNG E2E testing..."

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
    docker_build
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
    printf "\n"
    printf "Usage: %s [-i ip_family] [-b backend]\n" "$0"
    printf "\t-i set ip_family(ipv4/ipv6/dual) name in the e2e test runs.\n"
    printf "\t-b set backend (iptables/nft/ipvs) name in the e2e test runs.\n"
    printf "\t-c flag allows for ci_mode. Please don't run on local systems. \n"
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
