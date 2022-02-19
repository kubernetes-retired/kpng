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

###  Note for developers, if you want to install golang 1.18 ... you can run
###  eval "$(curl -sL https://raw.githubusercontent.com/travis-ci/gimme/master/gimme | GIMME_GO_VERSION=1.18beta1 bash)"
if [ $# -eq 0 ]
  then
    echo "please send a backend arg: iptables ipvs-as-sink nft windows/kernelspace"
fi

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
    echo "building $dir with GOOS='$2'"
    GOOS="$2" go build
    ls -altrh
  popd
}

function build_all_backends {
  cd ./backends
  for f in $(find -name go.mod); do build_package $(dirname $f); done
}

case $package in
  "iptables")  build_package backends/iptables linux ;;
  "ipvs")     build_package backends/ipvs-as-sink linux ;;
  "nft")      build_package backends/nft linux ;;
  "windows/kernelspace") build_package backends/windows/kernelspace windows ;;
  "")         build_all_backends ;;
  *)          echo "invalid argument: '$package'" ;;
esac

