package kernelspace

import (
	"net"
	"sync"

	"k8s.io/client-go/tools/events"
	//	"k8s.io/kubernetes/pkg/proxy"
	//	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"k8s.io/kubernetes/pkg/proxy/healthcheck"
	"k8s.io/kubernetes/pkg/util/async"
	//proxyconfig "k8s.io/kubernetes/pkg/proxy/config"

	"github.com/Microsoft/hcsshim/hcn"
	"sigs.k8s.io/kpng/api/localnetv1"
)

// Provider is a proxy interface enforcing services and windowsEndpoint methods
// implementations
type Provider interface {
	// OnEndpointsAdd is called whenever creation of new windowsEndpoint object
	// is observed.
	OnEndpointsAdd(ep *localnetv1.Endpoint, svc *localnetv1.Service)
	// OnEndpointsDelete is called whenever deletion of an existing windowsEndpoint
	// object is observed.
	OnEndpointsDelete(ep *localnetv1.Endpoint, svc *localnetv1.Service)
	// OnEndpointsSynced is called once all the initial event handlers were
	// called and the state is fully propagated to local cache.
	OnEndpointsSynced()
	// OnServiceAdd is called whenever creation of new service object
	// is observed.
	OnServiceAdd(service *localnetv1.Service)
	// OnServiceUpdate is called whenever modification of an existing
	// service object is observed.
	OnServiceUpdate(oldService, service *localnetv1.Service)
	// OnServiceDelete is called whenever deletion of an existing service
	// object is observed.
	OnServiceDelete(service *localnetv1.Service)
	// OnServiceSynced is called once all the initial event handlers were
	// called and the state is fully propagated to local cache.
	OnServiceSynced()

	// Sync immediately synchronizes the Provider's current state to proxy rules.
	Sync()
	// SyncLoop runs periodic work.
	// This is expected to run as a goroutine or as the main loop of the app.
	// It does not return.
	SyncLoop()
}

// Proxier (windows/kernelspace/Proxier) is copied impl of windows kernel based proxy for connections between a localhost:lport
// and services that provide the actual backends.
type Proxier struct {
	// TODO(imroc): implement node handler for winkernel proxier.
	//proxyconfig.NoopNodeHandler
	// endpointsChanges and serviceChanges contains all changes to windowsEndpoint and
	// services that happened since policies were synced. For a single object,
	// changes are accumulated, i.e. previous is state from before all of them,
	// current is state after applying all of those.
	endpointsChanges  *EndpointChangeTracker
	serviceChanges    *ServiceChangeTracker
	endPointsRefCount endPointsReferenceCountMap
	mu                sync.Mutex // protects the following fields
	serviceMap        ServiceMap
	endpointsMap      EndpointsMap
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

type endPointsReferenceCountMap map[string]*uint16

func (refCountMap endPointsReferenceCountMap) getRefCount(hnsID string) *uint16 {
	refCount, exists := refCountMap[hnsID]
	if !exists {
		refCountMap[hnsID] = new(uint16)
		refCount = refCountMap[hnsID]
	}
	return refCount
}
