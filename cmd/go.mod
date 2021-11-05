module sigs.k8s.io/kpng/cmd

go 1.17

replace (
	sigs.k8s.io/kpng => ../empty
	sigs.k8s.io/kpng/api => ../api
	sigs.k8s.io/kpng/backends/iptables => ../backends/iptables
	sigs.k8s.io/kpng/backends/ipvs-as-sink => ../backends/ipvs-as-sink
	sigs.k8s.io/kpng/backends/nft => ../backends/nft
	sigs.k8s.io/kpng/client => ../client
	sigs.k8s.io/kpng/server => ../server
)

replace (
	k8s.io/api => k8s.io/api v0.21.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.2
	k8s.io/client-go => k8s.io/client-go v0.21.2
	k8s.io/kubernetes => k8s.io/kubernetes v0.21.2
)

require (
	github.com/spf13/cobra v1.2.1
	google.golang.org/grpc v1.41.0
	google.golang.org/protobuf v1.27.1
	k8s.io/client-go v0.22.2
	k8s.io/klog v1.0.0
	sigs.k8s.io/kpng/api v0.0.0-20211016172202-2f0db1baddba
	sigs.k8s.io/kpng/backends/iptables v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/backends/ipvs-as-sink v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/backends/nft v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/client v0.0.0-20211016172202-2f0db1baddba
	sigs.k8s.io/kpng/server v0.0.0-00010101000000-000000000000
)
