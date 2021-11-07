#!/bin/bash

# TODO add license

# removing output from pushd and popd
pushd () {
  command pushd "$@" > /dev/null
}

popd () {
  command popd "$@" > /dev/null
}

function build_package() {
  local dir="$1"
  echo "trying to build '$1' "
  pushd ../$dir/
    go mod download
    go build
  popd
}

function build_iptables() {
  build_package backends/iptables
}

function build_ipvs() {
  build_package backends/ipvs-as-sink
}

function build_nft() {
  build_package backends/nft
}

function build_all_backends() {
  build_iptables
  build_ipvs
  build_nft
}

case $1 in
  "iptable")  build_iptable ;;
  "ipvs")     build_ipvs ;;
  "nft")      build_nft ;;
  "")         build_all_backends ;;
esac
