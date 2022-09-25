#!/usr/bin/env bash

set -xv
set -euo pipefail

GOOS=windows go build -trimpath -o ../dist
