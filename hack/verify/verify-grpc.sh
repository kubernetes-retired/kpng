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

# Verifying the protobuf

function verify {
    diff -q api/localnetv1/verify-protobuf/services.pb.go api/localnetv1/services.pb.go && diff -q api/localnetv1/verify-protobuf/services_grpc.pb.go api/localnetv1/services_grpc.pb.go || { echo "File doesn't match"; exit 1; }
}

function draw_line {
  echo "=================================="
}

draw_line
echo "Verifying grpc protobuf"
verify