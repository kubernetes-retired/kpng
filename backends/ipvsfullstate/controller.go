package ipvsfullsate

import (
	"fmt"
	"net/http"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/mux"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/ipsets"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/iptables"
	"sigs.k8s.io/kpng/backends/ipvsfullstate/internal/ipvs"
	"sigs.k8s.io/kpng/client"

	"k8s.io/klog/v2"

	"sync"
	"time"
)

// Controller handles the callbacks
type Controller struct {
	mu       sync.Mutex
	ipFamily v1.IPFamily

	ipsetList     map[string]*ipsets.Set
	ipvsManager   *ipvs.Manager
	ipsetsManager *ipsets.Manager
	iptManager    *iptables.Manager
}

const (
	readHeaderTimeout = time.Second * 5
)


func newController() Controller {
	return Controller{
		// ipvsManager manages virtual servers and destinations with linux kernel; leverage diffstore to avoid recreating objects
		ipvsManager: ipvs.NewManager(*IPVSSchedulingMethod, *IPVSDestinationWeight, "kube-ipvs0"),

		//// ipsetsManager manages virtual servers and destinations with linux kernel; leverage diffstore to avoid recreating objects
		ipsetsManager: ipsets.NewManager(),

		iptManager: iptables.NewManager(),
	}
}

// ServeProxyMode runs a HTTP listener for proxyMode detection.
func (c *Controller) ServeProxyMode(errCh chan error) {
	//TODO Get Bind address config. Time being leave it empty, kernel will choose loopback address 127.0.0.1
	bindAddress := "127.0.0.1:10249"
	proxyMode := "ipvs"
	proxyMux := mux.NewPathRecorderMux("kpng-ipvs")
	proxyMux.HandleFunc("/proxyMode", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = fmt.Fprintf(w, "%s", proxyMode)
	})

	fn := func() {
		server := &http.Server{
			Addr: bindAddress,
			Handler: proxyMux,
			ReadHeaderTimeout: readHeaderTimeout,
		}
		err := server.ListenAndServe()
		if err != nil {
			klog.Errorf("starting http server for proxyMode failed: %v", err)
			if errCh != nil {
				errCh <- err
			}
		}
	}
	go wait.Until(fn, 5*time.Second, wait.NeverStop)
}

func (c *Controller) Callback(ch <-chan *client.ServiceEndpoints) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// log time to process the callback
	st := time.Now()
	defer func() {
		klog.V(3).Infof("took %v to sync", time.Since(st).Truncate(time.Millisecond))
	}()

	// iterate over serviceEndpoints and simple request creation of network objects
	// to program the kernel, diffstores will maintain state and only create, update
	// and delete those which are required.
	for serviceEndpoints := range ch {
		switch serviceEndpoints.Service.Type {
		case ClusterIPService.String():
			c.addServiceEndpointsForClusterIP(serviceEndpoints)
		case NodePortService.String():
			c.addServiceEndpointsForNodePort(serviceEndpoints)
		case LoadBalancerService.String():
			c.addServiceEndpointsForLoadBalancer(serviceEndpoints)
		}

	}

	// compute the diffs
	c.ipvsManager.Done()
	c.ipsetsManager.Done()

	// execute the changes; this call will have actual side effects,
	// kernel will be programed to achieve the desired data path.
	c.ipvsManager.Apply()
	c.ipsetsManager.Apply()

	// reset the diffstore
	c.ipvsManager.Reset()
	c.ipsetsManager.Reset()

}
