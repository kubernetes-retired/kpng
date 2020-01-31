from mcluseau/golang-builder:1.13.7 as build
from alpine:3.11
entrypoint ["/bin/kube-proxy2"]
run apk add --update iptables iproute2
copy --from=build /go/bin/ /bin/
