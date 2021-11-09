#!/bin/bash

#TODO add license

# removing output from pushd
pushd () {
  command pushd "$@" > /dev/null
}
# removing output from popd
popd () {
  command popd "$@" > /dev/null
}

function draw_line() {
  echo "=================================="
}

if [ $(basename "$PWD") = "hack" ]
  then  
    cd ..
fi

draw_line
echo "resolving mod files"
draw_line
./hack/resolve_all_mod_files.sh

draw_line
echo "running all tests"
for f in $(find . -type f -name '*test.go' | sed -r 's|/[^/]+$||' |sort |uniq)
  do
    draw_line 
    echo "testing $f" 
    draw_line 
    (cd $f && go test -v) 
  done
