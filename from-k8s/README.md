Transitional module intended to control and reference imports from the k8s.io/kubernetes.

Depending on k8s.io/kubernetes is not a good practice, it's a huge package not meant to be imported.

Note de developers: please try to keep your imports to the minimum when copying from k/k, at the very least
don't copy the `*_test.go` files. Example if your kpng source is in `~/git/kpng`:

    p=pkg/apis/core ; mkdir -p ~/git/kpng/from-k8s/$p && rsync -r --exclude '*_test.go' $p/*.go ~/git/kpng/from-k8s/$p/

