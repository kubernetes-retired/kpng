module sigs.k8s.io/kpng/client

go 1.18

require (
	github.com/cespare/xxhash v1.1.0
	github.com/golang/protobuf v1.5.2
	github.com/google/btree v1.0.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	golang.org/x/exp v0.0.0-20220317015231-48e79f11773a
	google.golang.org/grpc v1.41.0
	google.golang.org/protobuf v1.27.1
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20220127004650-9b3446523e65
	sigs.k8s.io/kpng/api v0.0.0-00010101000000-000000000000
)

replace sigs.k8s.io/kpng/api => ../api

require (
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023 // indirect
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c // indirect
)

require (
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	golang.org/x/sys v0.0.0-20211019181941-9d821ace8654 // indirect
	golang.org/x/text v0.3.6 // indirect
)
