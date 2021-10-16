module sigs.k8s.io/kpng/cmd

go 1.17

replace (
	sigs.k8s.io/kpng => ../empty
	sigs.k8s.io/kpng/api => ../api
	sigs.k8s.io/kpng/backends => ../backends
	sigs.k8s.io/kpng/client => ../client
	sigs.k8s.io/kpng/server => ../server
)

replace (
	k8s.io/api => k8s.io/api v0.21.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.2
	k8s.io/kubernetes => k8s.io/kubernetes v0.21.2
)

require (
	github.com/spf13/cobra v1.2.1
	k8s.io/client-go v0.21.2
	k8s.io/klog v1.0.0
	sigs.k8s.io/kpng/backends v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/client v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/server v0.0.0-00010101000000-000000000000
)

require (
	google.golang.org/protobuf v1.27.1
	sigs.k8s.io/kpng/api v0.0.0-00010101000000-000000000000
)
