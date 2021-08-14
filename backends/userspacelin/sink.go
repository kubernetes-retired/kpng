package userspacelin

import (
	v1 "k8s.io/api/core/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	utilnode "k8s.io/kubernetes/pkg/util/node"
	"net"
	componentbaseconfig "k8s.io/component-base/config"

	"sigs.k8s.io/kpng/localsink"
	"sync"
	"github.com/spf13/pflag"
	"k8s.io/utils/exec"

	"sigs.k8s.io/kpng/backends/iptables"

	"sigs.k8s.io/kpng/localsink/decoder"
	"sigs.k8s.io/kpng/localsink/filterreset"
	"sigs.k8s.io/kpng/pkg/api/localnetv1"
	"k8s.io/klog/v2"

)

type Backend struct {
	localsink.Config
}

type Config {
	BindAddress string
}

// detectNodeIP returns the nodeIP used by the proxier
// The order of precedence is:
// 1. config.bindAddress if bindAddress is not 0.0.0.0 or ::
// 2. the primary IP from the Node object, if set
// 3. if no IP is found it defaults to 127.0.0.1 and IPv4
func detectNodeIP(client clientset.Interface, hostname, bindAddress string) net.IP {
	nodeIP := net.ParseIP(bindAddress)
	if nodeIP.IsUnspecified() {
		nodeIP = utilnode.GetNodeIP(client, hostname)
	}
	if nodeIP == nil {
		klog.V(0).Infof("can't determine this node's IP, assuming 127.0.0.1; if this is incorrect, please set the --bind-address flag")
		nodeIP = net.ParseIP("127.0.0.1")
	}
	return nodeIP
}

var wg = sync.WaitGroup{}
var usImpl map[v1.IPFamily]*UserspaceLinux
var hostname string
var _ decoder.Interface = &Backend{}

func New() *Backend {
	return &Backend{}
}

func (s *Backend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(s))
}

func (s *Backend) BindFlags(flags *pflag.FlagSet) {
}

// createClients creates a kube client and an event client from the given config and masterOverride.
// TODO remove masterOverride when CLI flags are removed.
func createClients(config componentbaseconfig.ClientConnectionConfiguration, masterOverride string) (clientset.Interface, v1core.EventsGetter, error) {
	var kubeConfig *rest.Config
	var err error

	if len(config.Kubeconfig) == 0 && len(masterOverride) == 0 {
		klog.InfoS("Neither kubeconfig file nor master URL was specified. Falling back to in-cluster config")
		kubeConfig, err = rest.InClusterConfig()
	} else {
		// This creates a client, first loading any specified kubeconfig
		// file, and then overriding the Master flag, if non-empty.
		kubeConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: config.Kubeconfig},
			&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: masterOverride}}).ClientConfig()
	}
	if err != nil {
		return nil, nil, err
	}

	kubeConfig.AcceptContentTypes = config.AcceptContentTypes
	kubeConfig.ContentType = config.ContentType
	kubeConfig.QPS = config.QPS
	kubeConfig.Burst = int(config.Burst)

	client, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	eventClient, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	return client, eventClient.CoreV1(), nil
}


func (s *Backend) Setup() {
	hostname = s.NodeName
	usImpl = make(map[v1.IPFamily]*UserspaceLinux)

	for _, protocol := range []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol} {
		usImpl := NewUserspaceLinux()

		usImpl.iptInterface = NewIPTableExec(exec.New(), Protocol(protocol))
		usImpl.serviceChanges = iptables.NewServiceChangeTracker(newServiceInfo, protocol, iptable.recorder)
		usImpl.endpointsChanges = NewEndpointChangeTracker(hostname, protocol, iptable.recorder)
		IptablesImpl[protocol] = iptable
	}
}

func (s *Backend) Reset() { /* noop, we're wrapped in filterreset */ }

func (s *Backend) Sync() {
	for _, impl := range IptablesImpl {
		wg.Add(1)
		go impl.sync()
	}
	wg.Wait()
}

func (s *Backend) SetService(svc *localnetv1.Service) {
	for _, impl := range IptablesImpl {
		impl.serviceChanges.Update(svc)
	}
}

func (s *Backend) DeleteService(namespace, name string) {
	for _, impl := range IptablesImpl {
		impl.serviceChanges.Delete(namespace, name)
	}
}

func (s *Backend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	for _, impl := range IptablesImpl {
		impl.endpointsChanges.EndpointUpdate(namespace, serviceName, key, endpoint)
	}
}

func (s *Backend) DeleteEndpoint(namespace, serviceName, key string) {
	for _, impl := range IptablesImpl {
		impl.endpointsChanges.EndpointUpdate(namespace, serviceName, key, nil)
	}
}