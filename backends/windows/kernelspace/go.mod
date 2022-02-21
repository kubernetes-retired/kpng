module sigs.k8s.io/kpng/backends/windows/kernelspace

go 1.18

replace (
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20220210182314-2f4923fbfbeb // indirect
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20220124181339-79836df3a7e5
	k8s.io/client-go => k8s.io/client-go v0.0.0-20220124173639-0f7ee7041f40
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20220124182644-6430128f81d0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20220124183052-e3974aa5a182
	k8s.io/code-generator => k8s.io/code-generator v0.23.4-rc.0
	k8s.io/component-base => k8s.io/component-base v0.0.0-20220124174242-95a6431a4277
	k8s.io/component-helpers => k8s.io/component-helpers v0.20.0-alpha.2.0.20220124174436-7f5c4cdf69dc
	k8s.io/controller-manager => k8s.io/controller-manager v0.20.0-alpha.1.0.20220124182419-3637f211e5d9
	k8s.io/cri-api => k8s.io/cri-api v0.23.4-rc.0.0.20220210224708-239ad2a1ff9c
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20220211100604-3399154d9e0d
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20220124175611-fd19b2824751
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20220124182853-288acdb5200e
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20220124181754-c440ad93b3b7
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20220124182218-cf91996069a4
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20220124184345-f830578aa6a2
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20220124182005-6b57f8141c2d
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20220128140858-951c08f77e35
	k8s.io/metrics => k8s.io/metrics v0.0.0-20210821163913-98d2fd1dc73d
	k8s.io/mount-utils => k8s.io/mount-utils v0.23.4-rc.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20220124180041-a4ffa70bddf6
)

replace (
	sigs.k8s.io/kpng => ../../../
	sigs.k8s.io/kpng/api => ../../../api
	sigs.k8s.io/kpng/client => ../../../client
)

// msft deps
require github.com/Microsoft/hcsshim v0.8.22

require (
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	google.golang.org/genproto v0.0.0-20210930144712-2e2e1008e8a3 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.23.4-rc.0
	k8s.io/client-go v0.0.0
	k8s.io/klog/v2 v2.30.0
	k8s.io/utils v0.0.0-20220127004650-9b3446523e65
	sigs.k8s.io/kpng/api v0.0.0-20211016163122-10ddff77b5bd
	sigs.k8s.io/kpng/client v0.0.0-20211016163122-10ddff77b5bd
)

require (
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/containerd/cgroups v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lithammer/dedent v1.1.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.11.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.28.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/sys v0.0.0-20211001092434-39dca1131b70 // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	google.golang.org/grpc v1.41.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/component-helpers v0.0.0 // indirect
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65 // indirect
	sigs.k8s.io/json v0.0.0-20211020170558-c049b76a60c6 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20220124172526-d42c342a4737
	k8s.io/apimachinery => k8s.io/apimachinery v0.23.4-rc.0
	// new stff added for windows not sure why...
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20220124175254-5a49cc5a8703
	k8s.io/kubernetes => k8s.io/kubernetes v1.24.0-alpha.0.0.20220211100604-c0add891584a
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.0-beta.0.0.20220124184544-0bfe2c41c1d4
//	k8s.io/component-helpers => k8s.io/component-helpers v0.22.2
)

require (
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	google.golang.org/appengine v1.6.7 // indirect
	k8s.io/apiserver v0.0.0
	k8s.io/component-base v0.0.0
	//k8s.io/apiserver v0.0.0
	//k8s.io/component-base v0.0.0
	k8s.io/kubernetes v0.0.0-00010101000000-000000000000
)
