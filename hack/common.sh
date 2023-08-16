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

# system data
NAMESPACE="kube-system"
CONFIG_MAP_NAME="kpng"
SERVICE_ACCOUNT_NAME="kpng"
CLUSTER_ROLE_NAME="system:node-proxier"
CLUSTER_ROLE_BINDING_NAME="kpng"

CLUSTER_CIDR_V4="10.1.0.0/16"
SERVICE_CLUSTER_IP_RANGE_V4="10.2.0.0/16"
CLUSTER_CIDR_V6="fd6d:706e:6701::/56"
SERVICE_CLUSTER_IP_RANGE_V6="fd6d:706e:6702::/112"
KPNG_SERVER_ADDRESS="unix:///k8s/proxy.sock"
KPNG_DEBUG_LEVEL=4

# kind
KIND_VERSION="v0.17.0"
# Users can specify docker.io, quay.io registry
KINDEST_NODE_IMAGE="docker.io/kindest/node"

CLUSTER_NAME="kpng-proxy"
K8S_VERSION="v1.25.3"
OS=$(uname| tr '[:upper:]' '[:lower:]')

GIT_COMMIT_HASH=$(git describe --tags --always --long)
GIT_REPO_URL=$(git config --get remote.origin.url)
# TODO: Create release tag
RELEASE=UNKNOWN

function add_to_path {
    ###########################################################################
    # Description:                                                            #
    # Add directory to path                                                   #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1:  directory                                                      #
    ###########################################################################
    [ $# -eq 1 ]
    if_error_exit "add_to $# Wrong number of arguments to ${FUNCNAME[0]}"

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
    # Copy binaries from the net to binaries directory                        #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: binary directory, path to where ginko will be installed         #
    #   arg2: Kubernetes version                                              #
    #   arg3: OS, name of the operating system                                #
    ###########################################################################

    [ $# -eq 3 ]
    if_error_exit "copybin $# Wrong number of arguments to ${FUNCNAME[0]}"

    local bin_directory="${1}"
    local k8s_version="${2}"
    local os="${3}"

    pushd "${0%/*}" > /dev/null || exit
          mkdir -p "${bin_directory}"
    popd > /dev/null || exit

    add_to_path "${bin_directory}"

    setup_kind "${bin_directory}"
    setup_kubectl "${bin_directory}" "${k8s_version}" "${os}"
    setup_ginkgo "${bin_directory}" "${k8s_version}" "${os}"
    setup_bpf2go "${bin_directory}"
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
    if_error_exit "setup_kind $# Wrong number of arguments to ${FUNCNAME[0]}"

    local install_directory=$1

    [ -d "${install_directory}" ]
    if_error_exit "Directory \"${install_directory}\" does not exist"


    if ! [ -f "${install_directory}"/kind ] ; then
        info_message -e "\nDownloading kind ..."

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
    #   arg1: installation directory, path to where kubectl will be installed #
    #   arg2: Kubernetes version                                              #
    #   arg3: OS, name of the operating system                                #
    ###########################################################################

    [ $# -eq 3 ]
    if_error_exit "setup_kubectl $# Wrong number of arguments to ${FUNCNAME[0]}"

    local install_directory=$1
    local k8s_version=${2}
    local os=${3}

    [ -d "${install_directory}" ]
    if_error_exit "Directory \"${install_directory}\" does not exist"


    if ! [ -f "${install_directory}"/kubectl ] ; then
        info_message -e "\nDownloading kubectl ..."

        local tmp_file=$(mktemp -q)
        if_error_exit "Could not create temp file, mktemp failed"

        curl -L https://dl.k8s.io/"${k8s_version}"/bin/"${os}"/amd64/kubectl -o "${tmp_file}"
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
    if_error_exit "ginkgo e2e.test $# Wrong number of arguments to ${FUNCNAME[0]}"

    local bin_dir=${1}
    local k8s_version=${2}
    local os=${3}
    local temp_directory=$(mktemp -qd)

    if ! [ -f "${bin_dir}"/ginkgo ] || ! [ -f "${bin_dir}"/e2e.test ] ; then
        info_message -e "\nDownloading ginkgo and e2e.test ..."
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

function setup_bpf2go() {
    ###########################################################################
    # Description:                                                            #
    # Install bpf2go binary                                                   #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: installation directory, path to where bpf2go will be installed  #
    ###########################################################################

    [ $# -eq 1 ]
    if_error_exit "bpf $# Wrong number of arguments to ${FUNCNAME[0]}"

    local install_directory=$1

    [ -d "${install_directory}" ]
    if_error_exit "Directory \"${install_directory}\" does not exist"

    if ! command_exists bpf2go ; then
        if ! command_exists go ; then
            echo "Dependency not met: 'bpf2go' not installed and cannot install with 'go'"
            exit 1
        fi

        [ -d "${install_directory}" ]
        if_error_exit "Directory \"${install_directory}\" does not exist"
	
        info_message "'bpf2go' not found, installing with 'go'"
	# set GOBIN to bin_directory to endure that binary is in search path
	export GOBIN=${install_directory}
	
	#remove GOPATH just to be sure
	export GOPATH=""
	
        go install github.com/cilium/ebpf/cmd/bpf2go@v0.9.2
        if_error_exit "cannot install bpf2go"

	pass_message "The tool bpf2go is installed. at: $(which bpf2go)"
    fi
}

function verify_sysctl_setting {
    ###########################################################################
    # Description:                                                            #
    # Verify that a sysctl attribute setting has a value                      #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: attribute                                                       #
    #   arg2: value                                                           #
    ###########################################################################
    [ $# -eq 2 ]
    if_error_exit "sysctl $# Wrong number of arguments to ${FUNCNAME[0]}"
    local attribute="${1}"
    local value="${2}"
    local result=$(sysctl -n  "${attribute}")
    if_error_exit "\"sysctl -n ${attribute}\" failed}"

    if [[ ! "${value}" -eq "${result}" ]] ; then
       echo "Failure: \"sysctl -n ${attribute}\" returned \"${result}\", not \"${value}\" as expected."
       exit
    fi
}

function set_sysctl {
    info_message "Setting sysctls"
    ###########################################################################
    # Description:                                                            #
    # Set a sysctl attribute to value                                         #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: attribute                                                       #
    #   arg2: value                                                           #
    ###########################################################################
    [ $# -eq 2 ]
    if_error_exit "set_sysctl $# Wrong number of arguments to ${FUNCNAME[0]}"
    local attribute="${1}"
    local value="${2}"
    local result=$(sysctl -n  "${attribute}")
    if_error_exit "\"sysctl -n ${attribute}\" failed"

    info_message "checking sysctls $value vs $result"
    if [[ ! "${value}" -eq "${result}" ]] ; then
       info_message "Setting: \"sysctl -w ${attribute}=${value}\""
       sudo sysctl -w  "${attribute}"="${value}"
       if_error_exit "\"sudo sysctl -w  ${attribute} = ${value}\" failed"
    fi
}

function verify_host_network_settings {
     ###########################################################################
     # Description:                                                            #
     # Verify hosts network settings                                           #
     #                                                                         #
     # Arguments:                                                              #
     #   arg1: ip_family                                                       #
     ###########################################################################
     [ $# -eq 1 ]
     if_error_exit "verify_host $# Wrong number of arguments to ${FUNCNAME[0]}"
     local ip_family="${1}"

     verify_sysctl_setting net.ipv4.ip_forward 1

     if [ "${ip_family}" = "ipv6" ]; then
       verify_sysctl_setting net.ipv6.conf.all.forwarding 1
       verify_sysctl_setting net.bridge.bridge-nf-call-arptables 0
       verify_sysctl_setting net.bridge.bridge-nf-call-ip6tables 0
       verify_sysctl_setting net.bridge.bridge-nf-call-iptables 0
     fi
}

function set_host_network_settings {
    ###########################################################################
    # Description:                                                            #
    # prepare hosts network settings                                          #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: ip_family                                                       #
    ###########################################################################
     [ $# -eq 1 ]
     if_error_exit "set_host $# Wrong number of arguments to ${FUNCNAME[0]}"
     local ip_family="${1}"

     set_sysctl net.ipv4.ip_forward 1
     if [ "${ip_family}" = "ipv6" ]; then
       set_sysctl net.ipv6.conf.all.forwarding 1
       set_sysctl net.bridge.bridge-nf-call-arptables 0
       set_sysctl net.bridge.bridge-nf-call-ip6tables 0
       set_sysctl net.bridge.bridge-nf-call-iptables 0
     fi
}

function compile_bpf {
    ###########################################################################
    # Description:                                                            #
    # compile bpf elf files for ebpf backend                                  #
    ###########################################################################

    pushd "${SCRIPT_DIR}"/../backends/ebpf > /dev/null || exit
    make bytecode
    if_error_exit "Failed to compile EBPF Programs"
    popd > /dev/null || exit
    pass_message "Compiled BPF programs"
}
