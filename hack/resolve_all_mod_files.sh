#!/bin/bash

#TODO add license

for f in $(find -name go.mod);
  do d=$(dirname $f); 
    echo "downloading mods in $d"; 
    ( cd $d && go mod download ); 
  done