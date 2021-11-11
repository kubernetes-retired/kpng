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

function is_kind_available() {
  if ! command -v kind &> /dev/null
  then
    echo "kind could not be found"
    exit
  fi
}

function get_sonobuoy() {
  mkdir sonobuoy
  this_os=$(uname| tr '[:upper:]' '[:lower:]')
  this_arcitecture=$(dpkg --print-architecture)
  url="https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.55.0/sonobuoy_0.55.0_"$this_os"_"$this_arcitecture".tar.gz"
  wget $url -O ./sonobuoy.tar.gz
  tar -xf sonobuoy.tar.gz -C ./sonobuoy
  rm sonobuoy.tar.gz
}

is_kind_available

# cd to dir of script
cd "${0%/*}"

get_sonobuoy

# create a kind cluster with kpng instead of kubeproxy
#./kpng-local-up.sh
