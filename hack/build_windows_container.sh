#!/bin/bash

# Copyright 2022 The Kubernetes Authors.
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


: ${REPOSITORY?"Need to REPOSITORY"}
TAG=${tag:-"latest"}

docker buildx create --name img-builder --use --platform windows/amd64
trap 'docker builx rm img-builder' EXIT

docker buildx build --platform windows/amd64 --output=type=registry --pull -f Dockerfile.Windows -t $REPOSITORY/kpng:$TAG .