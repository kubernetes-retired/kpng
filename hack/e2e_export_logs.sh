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

# this script is for ci
# if the test_e2e.sh fails, the logs still need to be exportet

set -e
pushd "${0%/*}" > /dev/null
E2E_DIR="$(pwd)/temp/e2e"
popd > /dev/null
CLUSTERNAME=$(kind get clusters | head -1)
E2E_FILE="${E2E_DIR}/${CLUSTERNAME}"
E2E_LOGS="${E2E_DIR}/artifacts/logs"
kind export logs --name="${CLUSTERNAME}" -v=3 "${E2E_LOGS}"
