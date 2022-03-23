module sigs.k8s.io/kpng/backends/windows/userspace

go 1.18

replace (
	k8s.io/api => k8s.io/api v0.22.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.4
	k8s.io/apiserver => k8s.io/apiserver v0.22.4 // indirect
	k8s.io/client-go => k8s.io/client-go v0.22.4
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.4
	k8s.io/code-generator => k8s.io/code-generator v0.22.4
	k8s.io/component-base => k8s.io/component-base v0.22.4
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.4
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.4
	k8s.io/cri-api => k8s.io/cri-api v0.22.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.4
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.4
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.4
	k8s.io/kubectl => k8s.io/kubectl v0.22.4
	k8s.io/kubelet => k8s.io/kubelet v0.22.4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.4
	k8s.io/metrics => k8s.io/metrics v0.22.4
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.4
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.4
)

replace (
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.4
	sigs.k8s.io/kpng => ../../empty
	sigs.k8s.io/kpng/api => ./../../../api
	sigs.k8s.io/kpng/client => ./../../../client
)

require sigs.k8s.io/kpng/client v0.0.0-00010101000000-000000000000

require (
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20220127004650-9b3446523e65
	sigs.k8s.io/kpng/api v0.0.0-00010101000000-000000000000
)

require (
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023 // indirect
	golang.org/x/sys v0.0.0-20211019181941-9d821ace8654 // indirect
	golang.org/x/text v0.3.6 // indirect
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c // indirect
	google.golang.org/grpc v1.41.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
)
