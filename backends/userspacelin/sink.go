package userspacelin

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/utils/exec"
	"sigs.k8s.io/kpng/backends/iptables"
	iptablesutil "sigs.k8s.io/kpng/backends/iptables/util"

	v1 "k8s.io/api/core/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	"net"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	componentbaseconfig "k8s.io/component-base/config"

	utilnode "sigs.k8s.io/kpng/backends/userspacelin/nodeutil"

	"sync"

	"github.com/spf13/pflag"
	//"k8s.io/utils/exec"

	//utilnet "k8s.io/apimachinery/pkg/util/net"
	klog "k8s.io/klog/v2"
	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/client/localsink/decoder"
	"sigs.k8s.io/kpng/client/localsink/filterreset"
)

type Backend struct {
	localsink.Config
}

type Config struct {
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
	// make a proxier for ipv4, ipv6

	usImpl = make(map[v1.IPFamily]*UserspaceLinux)
	usImpl[v1.IPv4Protocol].iptables = iptablesutil.NewIPTableExec(exec.New(), iptablesutil.Protocol(v1.IPv4Protocol))
	usImpl[v1.IPv6Protocol].iptables = iptablesutil.NewIPTableExec(exec.New(), iptablesutil.Protocol(v1.IPv6Protocol))

	nodePortAddresses := []string{}
	theProxier, _ := NewUserspaceLinux(NewLoadBalancerRR(), net.ParseIP("0.0.0.0"), usImpl[v1.IPv4Protocol].iptables, exec.New(), utilnet.PortRange{Base: 30000, Size: 2768}, time.Duration(1), time.Duration(1), time.Duration(1), nodePortAddresses)
	usImpl[v1.IPv4Protocol] = theProxier

	nodePortAddresses = []string{}
	// TODO THIS is untested...
	theProxier, _ = NewUserspaceLinux(NewLoadBalancerRR(), net.ParseIP("::/0"), usImpl[v1.IPv6Protocol].iptables, exec.New(), utilnet.PortRange{Base: 30000, Size: 2768}, time.Duration(1), time.Duration(1), time.Duration(1), nodePortAddresses)
	usImpl[v1.IPv4Protocol] = theProxier
}

func (s *Backend) Reset() { /* noop, we're wrapped in filterreset */ }

func (s *Backend) Sync() {
	for _, impl := range usImpl {
		wg.Add(1)
		go impl.syncProxyRules()
	}
	wg.Wait()
}

func (s *Backend) SetService(svc *localnetv1.Service) {
	for _, impl := range usImpl {
		impl.serviceChanges[types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}].Update(svc)
	}
}

func (s *Backend) DeleteService(namespace, name string) {
	for _, impl := range usImpl {
		impl.serviceChanges[types.NamespacedName{Namespace: namespace, Name: name}].Delete(namespace, name)

	}
}

// // updatePending updates a pending slice in the cache.
// func (cache *EndpointsCache) updatePending(svcKey types.NamespacedName, key string, endpoint *localnetv1.Endpoint) bool {
// 	var esInfoMap *endpointsInfoByName
// 	var ok bool
// 	if esInfoMap, ok = cache.trackerByServiceMap[svcKey]; !ok {
// 		esInfoMap = &endpointsInfoByName{}
// 		cache.trackerByServiceMap[svcKey] = esInfoMap
// 	}
// 	(*esInfoMap)[key] = endpoint
// 	return true
// }

// name of the endpoint is the same as the service name
func (s *Backend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	for _, impl := range usImpl {
		// TODO lookup the right data...
		// this can be looked up via the impl.serviceMap later... just iterate through the for loop for it and
		// match it to the serviceName/namespace
		//
		spn := []*iptables.ServicePortName{
			{
				NamespacedName: types.NamespacedName{Namespace: namespace, Name: serviceName},
				Port:           "8080",
				Protocol:       localnetv1.Protocol_TCP,
				PortName:       fmt.Sprintf("%v-%v", serviceName, 8080),
			},
		}

		impl.loadBalancer.OnEndpointsAdd(spn, endpoint)
	}
}

func (s *Backend) DeleteEndpoint(namespace, serviceName, key string) {
	print("this doesnt work sorry")
}

// 1
// 2 <-- last connection sent here
// 3
// -------------------- rr.OnEndpointsUpdate (1,2,3) (1,2,3,4)
// 1
// 2
// 3 <-- next connection will send here
// 4
// --------------------- rr.OnEndpointsUpdate (1,2,3) (1,2,4)
// 1
// 2
// 4 <-- next connection will send here
// ---------------------
// 1 <-- next connection will send here
// 2
// 4
