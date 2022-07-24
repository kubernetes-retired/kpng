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

package=$1

# removing output from pushd
function pushd {
  command pushd "$@" > /dev/null
}
# removing output from popd
function popd {
  command popd "$@" > /dev/null
}

function build_package {
  local dir="$1"
  echo "trying to build '$dir' "
  pushd ./$dir/
    go mod download
    go build
  popd
}

function build_all_backends {
  cd ./backends
  for f in $(find -name go.mod); do build_package $(dirname $f); done
}

case $package in
  "iptables")  build_package backends/iptables ;;
  "ipvs")     build_package backends/ipvs-as-sink ;;
  "nft")      build_package backends/nft ;;
  "ebpf")      build_package backends/ebpf ;;
  "")         build_all_backends ;;
  *)          echo "invalid argument: '$package'" ;;
esac
