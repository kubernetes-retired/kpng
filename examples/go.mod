module sigs.k8s.io/kpng/examples

go 1.17

replace (
	sigs.k8s.io/kpng/api => ../api
	sigs.k8s.io/kpng/client => ../client
	sigs.k8s.io/kpng/server => ../server
)

require (
	k8s.io/klog/v2 v2.20.0
	sigs.k8s.io/kpng/client v0.0.0-00010101000000-000000000000
)

require (
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/go-logr/logr v1.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/cobra v1.1.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023 // indirect
	golang.org/x/sys v0.0.0-20210616094352-59db8d763f22 // indirect
	golang.org/x/text v0.3.6 // indirect
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c // indirect
	google.golang.org/grpc v1.41.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	sigs.k8s.io/kpng/api v0.0.0-00010101000000-000000000000 // indirect
	sigs.k8s.io/kpng/server v0.0.0-00010101000000-000000000000 // indirect
)
