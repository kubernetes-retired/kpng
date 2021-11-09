#!/bin/bash

# TODO add license

package=$1

# removing output from pushd
pushd () {
  command pushd "$@" > /dev/null
}
# removing output from popd
popd () {
  command popd "$@" > /dev/null
}

function build_package() {
  local dir="$1"
  echo "trying to build '$dir' "
  pushd ../$dir/
    go mod download
    go build
  popd
}

function build_all_backends() {
  build_package backends/iptables
  build_package backends/ipvs-as-sink
  build_package backends/nft
}

case $package in
  "iptables")  build_package backends/iptables ;;
  "ipvs")     build_package backends/ipvs-as-sink ;;
  "nft")      build_package backends/nft ;;
  "")         build_all_backends ;;
  *)          echo "invalid argument: '$package'" ;;
esac
