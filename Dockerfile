from mcluseau/golang-builder:1.15.3 as build
from alpine:3.12
entrypoint ["/bin/kube-proxy2"]
run apk add --update iptables iproute2 nftables
copy --from=build /go/bin/ /bin/
