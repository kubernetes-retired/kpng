from alpine:3.16 as gomods

copy . /src/
run cd /src/ && find -type f \! \( -name go.work -o -name go.mod -o -name go.sum \) -exec rm {} +

from golang:1.19.2-alpine3.16 as build

# install dependencies
run apk add --update --no-cache \
    gcc musl-dev git \
    linux-headers libnl3-dev

# go mod args
arg GOPROXY
arg GONOSUMDB

arg RELEASE
arg GIT_REPO_URL
arg GIT_COMMIT_HASH

# cache dependencies, they don't change as much as the code
copy --from=gomods /src/ /src/

workdir /src
run go mod download

# test and build

add . ./
#run for f in $(find -name go.mod); do d=$(dirname $f); echo "testing in $d"; ( cd $d && go test ./... ); done
run go install -trimpath -buildvcs=false -ldflags "-X main.RELEASE=$RELEASE -X main.REPO=$GIT_REPO_URL -X main.COMMIT=$GIT_COMMIT_HASH" ./cmd/...

# the real image
from alpine:3.16
entrypoint ["/bin/kpng"]
run apk add --update iptables ip6tables iproute2 ipvsadm nftables ipset conntrack-tools
copy --from=build /go/bin/ /bin/
