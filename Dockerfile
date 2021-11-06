from alpine:3.13 as gomods

copy . /src/
run cd /src/ && find -type f \! \( -name go.mod -o -name go.sum \) -exec rm {} +

from golang:1.17.1-alpine3.13 as build

# install dependencies
run apk add --update --no-cache \
    gcc musl-dev \
    linux-headers libnl3-dev

# go mod args
arg GOPROXY
arg GONOSUMDB

# cache dependencies, they don't change as much as the code
copy --from=gomods /src/ /src/

workdir /src
run for f in $(find -name go.mod); do d=$(dirname $f); echo "downloading mods in $d"; ( cd $d && go mod download ); done

# test and build

add . ./
#run for f in $(find -name go.mod); do d=$(dirname $f); echo "testing in $d"; ( cd $d && go test ./... ); done
run cd cmd && go install -trimpath ./...

# the real image
from alpine:3.13
entrypoint ["/bin/kpng"]
run apk add --update iptables ip6tables iproute2 ipvsadm nftables ipset
copy --from=build /go/bin/ /bin/
