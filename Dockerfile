from mcluseau/golang-builder:1.16.3 as build
from alpine:edge
entrypoint ["/bin/kpng"]
run apk add --update iptables iproute2 ipvsadm nftables
copy --from=build /go/bin/ /bin/
