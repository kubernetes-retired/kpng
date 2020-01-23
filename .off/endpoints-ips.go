package main

import (
	"net"

	"k8s.io/klog"
	"github.com/mcluseau/kube-localnet-api/pkg/diffstore"
)

var endpointsIPsStore = diffstore.New()

type EndpointsIPs struct {
	IP           net.IP   `json:"ip"`
	External     bool     `json:"ext"`
	Destinations []net.IP `json:"dsts"`
}

func updateEndpointsIPs(change serviceEndpointsChange) {
	if change.Service == nil || change.Endpoints == nil {
		endpointsIPsStore.Delete(change.Prefix())
		return
	}

	targets := make([]net.IP, 0, len(change.Endpoints.Subsets))
	for _, subset := range change.Endpoints.Subsets {
		for _, addr := range subset.Addresses {
			ip := net.ParseIP(addr.IP)
			if ip == nil {
				// TODO log?
				continue
			}
			targets = append(targets, ip)
		}
	}

	tx := endpointsIPsStore.Begin(change.Prefix())

	for _, ipDef := range change.ServiceIPs() {
		tx.AddJSON(ipDef.IP.String(), &EndpointsIPs{
			IP:           ipDef.IP,
			External:     ipDef.External,
			Destinations: targets,
		})
	}

	logChanges("endpoints IPs", tx.Apply())
}

func logChanges(name string, changes diffstore.Changes) {
	if changes.Any() {
		klog.Infof("%s changes: %s", name, changes)
	}
}
