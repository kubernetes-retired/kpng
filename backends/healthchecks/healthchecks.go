/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package healthchecks

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/diffstore"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

type hcStatus struct {
	nsn   types.NamespacedName
	port  int
	count int
}

type leaf = diffstore.AnyLeaf[hcStatus]

var _ fullstate.Callback = (&backend{}).Callback

func (b *backend) Callback(ch <-chan *client.ServiceEndpoints) {
	defer b.statuses.Reset()

	for seps := range ch {
		port := int(seps.Service.HealthCheckNodePort)
		if port == 0 {
			continue // skip services without health check port
		}

		nsn := types.NamespacedName{
			Namespace: seps.Service.Namespace,
			Name:      seps.Service.Name,
		}
		key := nsn.Namespace + "/" + nsn.Name + ":" + strconv.Itoa(port)

		// count is the number of local endpoints
		count := 0
		for _, ep := range seps.Endpoints {
			if ep.Local {
				count++
			}
		}

		// check the leaf and act accordingly
		b.statuses.Get(key).Set(hcStatus{
			nsn:   nsn,
			port:  port,
			count: count,
		})
	}

	b.statuses.Done()

	for _, item := range b.statuses.Deleted() {
		key := item.Key()
		b.instances[key].stop()
		delete(b.instances, key)
	}

	for _, item := range b.statuses.Changed() {
		key := item.Key()
		status := item.Value().Get()

		if instance, ok := b.instances[key]; ok {
			instance.status = status

		} else {
			instance = &hcInstance{status: status}
			if instance.start(b) {
				b.instances[key] = instance
			}
		}
	}
}

type hcInstance struct {
	status   hcStatus
	listener net.Listener
}

func (hc *hcInstance) start(b *backend) (ok bool) {
	nsn := hc.status.nsn

	listener, err := net.Listen("tcp", b.ip+":"+strconv.Itoa(hc.status.port))

	if err != nil {
		// TODO send event, see events.EventRecorder and k8s.io/kubernetes/pkg/proxy/healthcheck.server#SyncServices
		klog.ErrorS(err, "Failed to start healthcheck", "node", b.cfg.NodeName, "service", nsn, "port", hc.status.port)
		return
	}

	ok = true
	hc.listener = listener

	go func() {
		klog.V(3).InfoS("Starting goroutine for healthcheck", "service", nsn, "address", hc.listener.Addr())
		err := http.Serve(hc.listener, hc)
		// reminder: err is always non-nil
		klog.ErrorS(err, "Healthcheck closed", "service", nsn, "address", hc.listener.Addr())
	}()

	return
}

func (hc *hcInstance) stop() {
	hc.listener.Close()
}

func (hc *hcInstance) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	s := hc.status

	resp.Header().Set("Content-Type", "application/json")
	resp.Header().Set("X-Content-Type-Options", "nosniff")

	if s.count == 0 {
		resp.WriteHeader(http.StatusServiceUnavailable)
	} else {
		resp.WriteHeader(http.StatusOK)
	}

	fmt.Fprintf(resp, `{
	"service": {
		"namespace": %q,
		"name": %q
	},
	"localEndpoints": %d
}`, s.nsn.Namespace, s.nsn.Name, s.count)
}
