from mcluseau/golang-builder:1.16.2 as build
from alpine:3.13
entrypoint ["/bin/kpng"]
run apk add --update iptables iproute2 nftables
copy --from=build /go/bin/ /bin/
