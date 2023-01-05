#!/bin/bash

echo "Hello Euclides!"

mkdir protoc 
# Would it be better or safe to work on the /tmp folder instead of protoc?
cd protoc 
curl -o protoc.zip -L https://github.com/protocolbuffers/protobuf/releases/download/v21.12/protoc-21.12-linux-x86_64.zip
unzip protoc.zip 
sudo cp bin/protoc /usr/local/bin 
sudo cp -r include/google /usr/local/include 
cd ../
sudo rm -r protoc
