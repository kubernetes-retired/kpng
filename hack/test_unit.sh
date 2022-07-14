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

# removing output from pushd
function pushd {
  command pushd "$@" > /dev/null
}
# removing output from popd
function popd {
  command popd "$@" > /dev/null
}

function draw_line {
  echo "=================================="
}

if [ $(basename "$PWD") = "hack" ]
  then
    cd ..
fi

draw_line
echo "resolving mod files"
draw_line
go work sync

draw_line
echo "running all tests"
hack/go-test-local-mods
