#!/bin/bash

echo "Installing Protoc"

# Protoc installation 
mkdir protoc 
# Would it be better or safe to work on the /tmp folder instead of protoc?
cd protoc 
curl -o protoc.zip -L https://github.com/protocolbuffers/protobuf/releases/download/v21.12/protoc-21.12-linux-x86_64.zip
unzip protoc.zip 
sudo cp bin/protoc /usr/local/bin 
sudo cp -r include/google /usr/local/include 
cd ../
sudo rm -r protoc 

# Libraries 
echo "Installing libnl-3-dev and libnl-genl-3-dev libraries"
sudo apt-get install libnl-3-dev #Fixes fatal error: netlink/netlink.h: No such file or directory
sudo apt-get install libnl-genl-3-dev #Fixes /usr/bin/ld: cannot find -lnl-genl-3
