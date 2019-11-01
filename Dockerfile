from mcluseau/golang-builder:1.12.7 as build
from scratch
entrypoint ["/bin/kube-localnet-api"]
copy --from=build /go/bin/ /bin/
