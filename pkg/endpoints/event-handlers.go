package endpoints

import (
	"k8s.io/client-go/tools/cache"

	"github.com/mcluseau/kube-proxy2/pkg/proxystore"
)

type eventHandler struct {
	s        *proxystore.Store
	syncSet  bool
	informer cache.SharedIndexInformer
}

func (h *eventHandler) updateSync(set proxystore.Set, tx *proxystore.Tx) {
	if h.syncSet {
		return
	}

	if h.informer.HasSynced() {
		tx.SetSync(set)
		h.syncSet = true
	}
}
