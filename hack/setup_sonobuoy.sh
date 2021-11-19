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

function is_kind_available {
  if ! command -v kind &> /dev/null
  then
    echo "kind could not be found"
    exit
  fi
}

function setup_sonobuoy {
  if [ -f ./sonobuoy/sonobuoy ]; then
    echo "Found Sonobuoy, skipping setup."
  else 
    mkdir sonobuoy
    os=$(uname| tr '[:upper:]' '[:lower:]')
    architecture=amd64 #TODO add logic to determine architecture
    url="https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.55.0/sonobuoy_0.55.0_"$os"_"$architecture".tar.gz"
    wget $url -O ./sonobuoy.tar.gz
    tar -xf sonobuoy.tar.gz -C ./sonobuoy
    rm sonobuoy.tar.gz
  fi
}

function setup_kind {
  if kind get clusters | grep -q kpng-proxy; then
    echo "Found kind cluster 'kpng-proxy', skipping setup."
  else
    ./kpng-local-up.sh
  fi
}

is_kind_available

# cd to dir of script
cd "${0%/*}"

setup_sonobuoy
setup_kind
