#!/usr/bin/env bash

set -xv
set -euo pipefail

docker run --rm \
  -v "$PWD"/../../../:/src \
  -w /src \
  -e GOOS=windows -e GOARCH=386 \
  golang:1.19 go build -v -o ./backends/windows/kernelspace/kpng-windows-386.exe ./backends/windows/kernelspace
