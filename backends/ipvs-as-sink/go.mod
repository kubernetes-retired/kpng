module sigs.k8s.io/kpng/backends/ipvs-as-sink

go 1.18

replace (
	sigs.k8s.io/kpng/api => ../../api
	sigs.k8s.io/kpng/backends/iptables => ../iptables
	sigs.k8s.io/kpng/client => ../../client
)

replace (
	k8s.io/api => k8s.io/api v0.21.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.2
	k8s.io/kubernetes => k8s.io/kubernetes v0.21.2
)

require (
	github.com/google/seesaw v0.0.0-20210205180622-915f447b8ed8
	github.com/lithammer/dedent v1.1.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/sys v0.0.0-20211019181941-9d821ace8654
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20220127004650-9b3446523e65
	sigs.k8s.io/kpng/api v0.0.0-20211016172202-2f0db1baddba
	sigs.k8s.io/kpng/backends/iptables v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kpng/client v0.0.0-20211016172202-2f0db1baddba
)

require (
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	golang.org/x/net v0.0.0-20210929193557-e81a3d93ecf6 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20210930144712-2e2e1008e8a3 // indirect
	google.golang.org/grpc v1.41.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/klog/v2 v2.20.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
)
