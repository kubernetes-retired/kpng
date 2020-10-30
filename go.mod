module github.com/mcluseau/kube-proxy2

go 1.13

require (
	github.com/OneOfOne/xxhash v1.2.7 // indirect
	github.com/cespare/xxhash v1.1.0
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.3
	github.com/google/btree v1.0.0
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.3 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/zevenet/kube-nftlb v0.1.1-0.20201027100916-c95119c4e332
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897 // indirect
	golang.org/x/net v0.0.0-20201029221708-28c70e62bb1d // indirect
	golang.org/x/sys v0.0.0-20201029080932-201ba4db2418 // indirect
	golang.org/x/text v0.3.4 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20201029200359-8ce4113da6f7 // indirect
	google.golang.org/grpc v1.33.1
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.4-rc.0
	k8s.io/client-go v1.5.1
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.4.0 // indirect
	k8s.io/utils v0.0.0-20201027101359-01387209bb0d // indirect
)

replace (
	github.com/googleapis/gnostic v0.4.1 => github.com/googleapis/gnostic v0.3.1
	k8s.io/client-go => k8s.io/client-go v0.19.3
)
