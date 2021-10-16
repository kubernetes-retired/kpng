module sigs.k8s.io/kpng/backends/ipvs

go 1.17

replace (
	sigs.k8s.io/kpng => ../../empty
	sigs.k8s.io/kpng/api => ../../api
	sigs.k8s.io/kpng/client => ../../client
)

require (
	github.com/spf13/pflag v1.0.5
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/net v0.0.0-20210929193557-e81a3d93ecf6 // indirect
	golang.org/x/sys v0.0.0-20211001092434-39dca1131b70
	google.golang.org/genproto v0.0.0-20210930144712-2e2e1008e8a3 // indirect
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.20.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/kpng/api v0.0.0-20211016163122-10ddff77b5bd
	sigs.k8s.io/kpng/client v0.0.0-00010101000000-000000000000
)

require (
	github.com/go-logr/logr v1.1.0 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	golang.org/x/text v0.3.7 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.21.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.2
	k8s.io/kubernetes => k8s.io/kubernetes v0.21.2
)

require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/cobra v1.2.1 // indirect
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df // indirect
	google.golang.org/grpc v1.41.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
)
