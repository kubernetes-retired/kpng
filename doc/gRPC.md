## This document will help to build the gRPC stubs.

### Step 1: Install the [protoc binaries](https://grpc.io/docs/protoc-installation/) under Linux

```
$ apt install -y protobuf-compiler
$ protoc --version  # Ensure compiler version is 3+
```
### Step 2: Install Go protocol buffers plugin and update the GO PATH:

```
$ go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
$ go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

$ export PATH="$PATH:$(go env GOPATH)/bin"
```

### Step 3. Generate stubs using proto file

```
protoc -I ./ --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. $(find . -name '*.proto')
```

