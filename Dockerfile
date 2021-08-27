from golang:1.17.0-alpine3.13 as build

# install dependencies
run apk add --update --no-cache \
    gcc musl-dev \
    linux-headers libnl3-dev

workdir /src

# go mod args
arg GOPROXY
arg GONOSUMDB

# cache dependencies, they don't change as much as the code
add go.mod go.sum ./
run go mod download

# test and build

add . ./
run go test ./...
run go install -trimpath ./cmd/...

# the real image
from alpine:3.13
entrypoint ["/bin/kpng"]
run apk add --update iptables ip6tables iproute2 ipvsadm nftables
copy --from=build /go/bin/ /bin/
