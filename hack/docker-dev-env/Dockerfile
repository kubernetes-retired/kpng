from golang:1.19.4-alpine3.17

# system dependencies
run apk add --update --no-cache \
    gcc musl-dev git curl sudo \
    linux-headers libnl3-dev

env GOBIN=/usr/local/bin

run go install github.com/cortesi/modd/cmd/modd@latest \
 && rm -fr /go/pkg

# gRPC dependencies
run apk add --no-cache protobuf=3.21.9-r0
run go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1 \
 && rm -fr /go/pkg
run go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0 \
 && rm -fr /go/pkg

run apk add vim

# entrypoint
env GOBIN=/go/bin
entrypoint ["/entrypoint.sh"]
copy entrypoint.sh /
