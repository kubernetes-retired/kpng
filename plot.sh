#! /bin/sh

input="${1}"
output="${2:-${1%.*}.svg}"

set -x
gnuplot -e "output='${output}'" -e "input='${input}'" cpu-mem.gnuplot
