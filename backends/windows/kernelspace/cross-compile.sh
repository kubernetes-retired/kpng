#!/usr/bin/env bash

set -xv
set -euo pipefail

docker run --rm \
  -v "$PWD"/../../../:/tmp/kpng \
  -w /tmp/kpng/backends/windows/kernelspace \
  -e GOOS=windows -e GOARCH=386 \
  golang:1.18 go build -v -o myapp-windows-386.exe
