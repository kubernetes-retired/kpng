package metrics

import (
	"context"
	"fmt"
	"time"

	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/klog/v2"
)

const (
	readHeaderTimeout = time.Second * 5
)

var Kpng_k8s_api_events = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "kpng_k8s_api_events_total",
	Help: "The total number of received events from the Kubernetes API",
})

var Kpng_node_local_events = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "kpng_node_local_events_total",
	Help: "The total number of received events from the Kubernetes API for a given node",
})

// StartMetricsServer runs the prometheus listener so that KPNG metrics can be collected
// TODO add TLS Auth if configured
func StartMetricsServer(bindAddress string,
	stopChan <-chan struct{}) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	klog.Infof("Starting metrics server at %s", bindAddress)

	go func() {
		var server *http.Server
		go utilwait.Until(func() {
			var err error
			server = &http.Server{
				Addr:              bindAddress,
				Handler:           mux,
				ReadHeaderTimeout: readHeaderTimeout,
			}
			err = server.ListenAndServe()

			if err != nil && err != http.ErrServerClosed {
				utilruntime.HandleError(fmt.Errorf("starting metrics server failed: %v", err))
			}
		}, 5*time.Second, stopChan)

		<-stopChan
		klog.Infof("Stopping metrics server %s", server.Addr)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			klog.Errorf("Error stopping metrics server: %v", err)
		}
	}()
}
