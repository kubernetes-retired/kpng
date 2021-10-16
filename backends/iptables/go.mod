module sigs.k8s.io/kpng/backends/iptables

go 1.17

replace (
	k8s.io/apiserver => k8s.io/apiserver v0.22.2 // indirect
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.2
	k8s.io/client-go => k8s.io/client-go v0.22.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.2
	k8s.io/code-generator => k8s.io/code-generator v0.22.2
	k8s.io/component-base => k8s.io/component-base v0.22.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.2
	k8s.io/cri-api => k8s.io/cri-api v0.22.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.2
	k8s.io/kubectl => k8s.io/kubectl v0.22.2
	k8s.io/kubelet => k8s.io/kubelet v0.22.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.2
	k8s.io/metrics => k8s.io/metrics v0.0.0-20210821163913-98d2fd1dc73d
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.2
)

replace (
	sigs.k8s.io/kpng => ../../empty
	sigs.k8s.io/kpng/api => ../../api
	sigs.k8s.io/kpng/client => ../../client
)

require (
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20210929193557-e81a3d93ecf6 // indirect
	golang.org/x/sys v0.0.0-20211001092434-39dca1131b70
	google.golang.org/genproto v0.0.0-20210930144712-2e2e1008e8a3 // indirect
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/klog/v2 v2.20.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/kpng/api v0.0.0-20211016163122-10ddff77b5bd
	sigs.k8s.io/kpng/client v0.0.0-20211016163122-10ddff77b5bd
)

require (
	github.com/go-logr/logr v1.1.0 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	golang.org/x/text v0.3.7 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.22.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.2
	k8s.io/kubernetes => k8s.io/kubernetes v0.22.2
)

require (
	golang.org/x/oauth2 v0.0.0-20210402161424-2e8d93401602 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	k8s.io/component-base v0.0.0-00010101000000-000000000000
)
