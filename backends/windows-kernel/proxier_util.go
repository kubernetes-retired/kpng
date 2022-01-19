package winkernel

import (
	"net"
	"sync"

	"k8s.io/kubernetes/pkg/proxy"
	"k8s.io/kubernetes/pkg/util/async"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"k8s.io/client-go/tools/events"
	proxyconfig "k8s.io/kubernetes/pkg/proxy/config"

	"github.com/Microsoft/hcsshim/hcn"
)

// Proxier is an hns based proxy for connections between a localhost:lport
// and services that provide the actual backends.
type Proxier struct {
	// TODO(imroc): implement node handler for winkernel proxier.
	proxyconfig.NoopNodeHandler

	// endpointsChanges and serviceChanges contains all changes to endpoints and
	// services that happened since policies were synced. For a single object,
	// changes are accumulated, i.e. previous is state from before all of them,
	// current is state after applying all of those.
	endpointsChanges  *proxy.EndpointChangeTracker
	serviceChanges    *proxy.ServiceChangeTracker
	endPointsRefCount endPointsReferenceCountMap
	mu                sync.Mutex // protects the following fields
	serviceMap        proxy.ServiceMap
	endpointsMap      proxy.EndpointsMap
	// endpointSlicesSynced and servicesSynced are set to true when corresponding
	// objects are synced after startup. This is used to avoid updating hns policies
	// with some partial data after kube-proxy restart.
	endpointSlicesSynced bool
	servicesSynced       bool
	isIPv6Mode           bool
	initialized          int32
	syncRunner           *async.BoundedFrequencyRunner // governs calls to syncProxyRules
	// These are effectively const and do not need the mutex to be held.
	masqueradeAll  bool
	masqueradeMark string
	clusterCIDR    string
	hostname       string
	nodeIP         net.IP
	recorder       events.EventRecorder

	serviceHealthServer healthcheck.ServiceHealthServer
	healthzServer       healthcheck.ProxierHealthUpdater

	// Since converting probabilities (floats) to strings is expensive
	// and we are using only probabilities in the format of 1/n, we are
	// precomputing some number of those and cache for future reuse.
	precomputedProbabilities []string

	hns               HostNetworkService
	network           hnsNetworkInfo
	sourceVip         string
	hostMac           string
	isDSR             bool
	supportedFeatures hcn.SupportedFeatures
}
