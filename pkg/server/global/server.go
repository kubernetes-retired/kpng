package global

import (
	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
	"m.cluseau.fr/kube-proxy2/pkg/diffstore"
	"m.cluseau.fr/kube-proxy2/pkg/proxystore"
	"m.cluseau.fr/kube-proxy2/pkg/server/watchstate"
)

type Server struct {
	Store *proxystore.Store
}

var syncItem = &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{}}

func (s *Server) Watch(res localnetv1.Global_WatchServer) error {
	var rev uint64

	w := watchstate.New(res, proxystore.AllSets)

	for {
		if _, err := res.Recv(); err != nil {
			return err
		}

		rev = s.Store.View(rev, func(tx *proxystore.Tx) {
			if !tx.AllSynced() {
				return
			}

			// sync all stores
			for _, set := range proxystore.AllSets {
				diff := w.StoreFor(set)
				tx.Each(set, func(kv *proxystore.KV) bool {
					h := kv.Value.GetHash()
					diff.Set([]byte(kv.Path()), h, kv.Value)
					return true
				})
			}
		})

		w.SendUpdates(localnetv1.Set_GlobalServiceInfos)
		w.SendUpdates(localnetv1.Set_GlobalNodeInfos)
		w.SendUpdates(localnetv1.Set_GlobalEndpointInfos)

		w.SendDeletes(localnetv1.Set_GlobalEndpointInfos)
		w.SendDeletes(localnetv1.Set_GlobalNodeInfos)
		w.SendDeletes(localnetv1.Set_GlobalServiceInfos)

		res.Send(syncItem)

		for _, set := range proxystore.AllSets {
			w.StoreFor(set).Reset(diffstore.ItemDeleted)
		}
	}
}
