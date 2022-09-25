#!/bin/bash

if [ $# -ne 1 ]
  then
    echo "No libbpf version provided"
    exit 1
fi
echo "Cloning libbpf from source, version $1"
mkdir -p libbpf
# We only need the uapi and core helpers so use a sparse checkout
cd libbpf
git init
git config core.sparseCheckout true
echo /include/uapi/linux > .git/info/sparse-checkout
echo /src >> .git/info/sparse-checkout
if git remote add -f origin https://github.com/libbpf/libbpf
then
    git pull origin $1
else
    git fetch && git reset --hard tags/$1
fi
cd src
# install headers in a nested bpf dir so we can have clean import paths in the bpf C program
make INCLUDEDIR="./" install_headers
cd ..
cd ..
