#! /bin/sh
gnuplot -e "output='${1%.*}.svg'" -e "input='$1'" cpu-mem.gnuplot
