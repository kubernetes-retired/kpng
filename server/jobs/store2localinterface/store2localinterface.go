/*
Copyright 2021 The Kubernetes Authors.

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

package store2localinterface

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/server/jobs/kube2store"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/diffstore"
	"sigs.k8s.io/kpng/server/pkg/endpoints"
	"sigs.k8s.io/kpng/server/pkg/proxy"
	"sigs.k8s.io/kpng/server/proxystore"
	"sigs.k8s.io/kpng/server/serde"
)

type SendNodeLocalState struct {
	consumer      NodeLocalStateConsumer
	nodeName      string
	ServicesDiff  *diffstore.Store[string, *diffstore.JSONLeaf[*localnetv1.Service]]
	EndpointsDiff *diffstore.Store[string, *diffstore.JSONLeaf[*localnetv1.EndpointInfo]]
	Store         *proxystore.Store
}

type NodeLocalStateConsumer interface {
	UpdateServices(services <-chan *localnetv1.Service)
	DeleteServices(services <-chan *localnetv1.Service)
	UpdateEndpoints(endpoints <-chan *localnetv1.EndpointInfo)
	DeleteEndpoints(endpoints <-chan *localnetv1.EndpointInfo)
}

func (s *SendNodeLocalState) Update(tx *proxystore.Tx) {
	if !tx.AllSynced() {
		return
	}

	nodeName := s.nodeName

	// set all new values
	tx.Each(proxystore.Services, func(kv *proxystore.KV) bool {
		key := kv.Namespace + "/" + kv.Name
		s.ServicesDiff.Get(key).Set(kv.Service.Service)

		// filter endpoints for this node
		endpointInfos := endpoints.ForNode(tx, kv.Service, nodeName)

		for _, ei := range endpointInfos {
			// hash only the endpoint
			hash := serde.Hash(ei.Endpoint)

			var epKey string
			if ei.PodName == "" {
				// key is service key + endpoint hash (64 bits, in hex)
				epKey = fmt.Sprintf("%s/%d", key, hash)
			} else {
				// key is service key + podName
				epKey = fmt.Sprintf("%s/%s", key, ei.PodName)
			}

			s.EndpointsDiff.Get(epKey).Set(ei)
		}

		return true
	})
}

func (j *SendNodeLocalState) RunNodeLocalStateConsumer(ctx context.Context) (err error) {
	var (
		rev    uint64
		closed bool
	)

	for {
		if err = ctx.Err(); err != nil {
			// check the context is still active; we expect the watch() to fail fast in this case
			return
		}

		updated := false
		for !updated {
			// update the state
			rev, closed = j.Store.View(rev, func(tx *proxystore.Tx) {
				j.Update(tx)
			})

			if closed {
				return
			}

			j.ServicesDiff.Done()
			j.EndpointsDiff.Done()

			// send the diff
			updated = j.SendDiff()

			// Reset for next rev
			j.ServicesDiff.Reset()
			j.EndpointsDiff.Reset()
		}

	}
}

func (j *SendNodeLocalState) SendDiff() (updated bool) {
	var wg sync.WaitGroup
	var count uint64
	var updatedServicesChan, deletedServicesChan chan *localnetv1.Service
	var updatedEndpointsChan, deletedEndpointsChan chan *localnetv1.EndpointInfo

	wg.Add(1)
	go func() {
		defer wg.Done()

		updatedServices := j.ServicesDiff.Changed()
		updatedServicesCount := len(updatedServices)

		updatedServicesChan = make(chan *localnetv1.Service, updatedServicesCount)
		defer close(updatedServicesChan)

		if updatedServicesCount == 0 {
			klog.V(2).Info("No Service Update Events")
			return
		}
		atomic.AddUint64(&count, uint64(updatedServicesCount))

		for _, service := range updatedServices {
			updatedServicesChan <- service.Value().Get()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		deletedServices := j.ServicesDiff.Deleted()
		deletedServicesCount := len(deletedServices)

		deletedServicesChan = make(chan *localnetv1.Service, deletedServicesCount)
		defer close(deletedServicesChan)

		if deletedServicesCount == 0 {
			klog.V(2).Info("No Service Delete Events")
			return
		}
		atomic.AddUint64(&count, uint64(deletedServicesCount))

		for _, service := range deletedServices {
			deletedServicesChan <- service.Value().Get()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		updatedEndpoints := j.EndpointsDiff.Changed()
		updatedEndpointsCount := len(updatedEndpoints)

		updatedEndpointsChan = make(chan *localnetv1.EndpointInfo, len(updatedEndpoints))
		defer close(updatedEndpointsChan)

		if updatedEndpointsCount == 0 {
			klog.V(2).Info("No Endpoint Update Events")
			return
		}
		atomic.AddUint64(&count, uint64(updatedEndpointsCount))

		for _, service := range updatedEndpoints {
			updatedEndpointsChan <- service.Value().Get()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		deletedEndpoints := j.EndpointsDiff.Deleted()
		deletedEndpointsCount := len(deletedEndpoints)

		deletedEndpointsChan = make(chan *localnetv1.EndpointInfo, len(deletedEndpoints))
		defer close(deletedEndpointsChan)

		if deletedEndpointsCount == 0 {
			klog.V(2).Info("No Endpoint Delete Events")
			return
		}

		atomic.AddUint64(&count, uint64(deletedEndpointsCount))

		for _, endpoint := range deletedEndpoints {
			deletedEndpointsChan <- endpoint.Value().Get()
		}
	}()

	wg.Wait()

	j.consumer.UpdateServices(updatedServicesChan)
	j.consumer.DeleteServices(deletedServicesChan)
	j.consumer.UpdateEndpoints(updatedEndpointsChan)
	j.consumer.DeleteEndpoints(deletedEndpointsChan)

	klog.V(2).Infof("All routines to send diff to consumers are done logged %d total changes", count)
	return count != 0
}

func WatchStore(ctx context.Context, store *proxystore.Store, nodeName string, consumer NodeLocalStateConsumer) {

	if nodeName == "" {
		var err error
		nodeName, err = os.Hostname()
		if err != nil {
			klog.Errorf("Failed to get hostname: %v", err)
		}
	}

	job := &SendNodeLocalState{
		consumer:      consumer,
		nodeName:      nodeName,
		ServicesDiff:  diffstore.NewJSONStore[string, *localnetv1.Service](),
		EndpointsDiff: diffstore.NewJSONStore[string, *localnetv1.EndpointInfo](),
		Store:         store,
	}

	job.RunNodeLocalStateConsumer(ctx)
}

func StartKube2store(Config *kube2store.Config, kubeConfig string, kubeServer string) (ctx context.Context, store *proxystore.Store, err error) {
	ctx, cancel := context.WithCancel(context.Background())

	// handle exit signals
	go func() {
		proxy.WaitForTermSignal()
		cancel()

		proxy.WaitForTermSignal()
		klog.Fatal("forced exit after second term signal")
		os.Exit(1)
	}()

	// setup k8s client
	if kubeConfig == "" {
		kubeConfig = os.Getenv("KUBECONFIG")
	}

	cfg, err := clientcmd.BuildConfigFromFlags(kubeServer, kubeConfig)
	if err != nil {
		err = fmt.Errorf("error building kubeconfig: %w", err)
		return
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		err = fmt.Errorf("error building kubernetes clientset: %w", err)
		return
	}

	// create the store
	store = proxystore.New()

	// start kube2store
	go kube2store.Job{
		Kube:   kubeClient,
		Store:  store,
		Config: Config,
	}.Run(ctx)

	return
}
