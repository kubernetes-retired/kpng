module sigs.k8s.io/kpng/backends/ipvs-as-sink

go 1.17

replace (
	sigs.k8s.io/kpng/api => ../../api
	sigs.k8s.io/kpng/client => ../../client
)

replace (
	k8s.io/api => k8s.io/api v0.21.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.2
	k8s.io/kubernetes => k8s.io/kubernetes v0.21.2
)

require (
	github.com/google/seesaw v0.0.0-20210205180622-915f447b8ed8
	github.com/spf13/pflag v1.0.5
	github.com/vishvananda/netlink v1.1.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.20.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/kpng/api v0.0.0-20211016172202-2f0db1baddba
	sigs.k8s.io/kpng/backends/ipvs v0.0.0-20211016172202-2f0db1baddba
	sigs.k8s.io/kpng/client v0.0.0-20211016172202-2f0db1baddba
)

require (
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
)
