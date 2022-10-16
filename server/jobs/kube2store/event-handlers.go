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

package kube2store

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/server/proxystore"
)

type eventHandler struct {
	config    *Config
	s         *proxystore.Store
	informer  cache.SharedIndexInformer
	isSyncSet bool
}

func (h *eventHandler) updateSync(set proxystore.Set, tx *proxystore.Tx) {

	if h.isSyncSet {
		// this happens 5x a second, so definetly V(2)
		klog.V(2).Info("updateSync: Skipping")
		return
	}

	if h.informer.HasSynced() {
		klog.V(2).Info("updateSync: Skipping")
		tx.SetSync(set)
		h.isSyncSet = true
	}
}
