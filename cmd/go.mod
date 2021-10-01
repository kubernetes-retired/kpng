module sigs.k8s.io/kpng/cmd

go 1.17

replace (
	sigs.k8s.io/kpng => ../empty
	sigs.k8s.io/kpng/api => ../api
	sigs.k8s.io/kpng/backends/iptables => ../backends/iptables
	sigs.k8s.io/kpng/backends/ipvs => ../backends/ipvs
	sigs.k8s.io/kpng/backends/ipvs-as-sink => ../backends/ipvs-as-sink
	sigs.k8s.io/kpng/backends/nft => ../backends/nft
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
	google.golang.org/protobuf v1.27.1
	k8s.io/client-go v0.21.2
	k8s.io/klog v1.0.0
	sigs.k8s.io/kpng/api v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/backends/ipvs v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/backends/ipvs-as-sink v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/backends/nft v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/client v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/server v0.0.0-00010101000000-000000000000
)

require (
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.1.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/seesaw v0.0.0-20210205180622-915f447b8ed8 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/vishvananda/netlink v1.1.0 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	golang.org/x/net v0.0.0-20210929193557-e81a3d93ecf6 // indirect
	golang.org/x/oauth2 v0.0.0-20210427180440-81ed05c6b58c // indirect
	golang.org/x/sys v0.0.0-20211001092434-39dca1131b70 // indirect
	golang.org/x/term v0.0.0-20210429154555-c04ba851c2a4 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210930144712-2e2e1008e8a3 // indirect
	google.golang.org/grpc v1.41.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.21.2 // indirect
	k8s.io/apimachinery v0.22.2 // indirect
	k8s.io/klog/v2 v2.20.0 // indirect
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)
