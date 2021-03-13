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

package global

import (
	"m.cluseau.fr/kpng/pkg/api/localnetv1"
	"m.cluseau.fr/kpng/pkg/diffstore"
	"m.cluseau.fr/kpng/pkg/proxystore"
	"m.cluseau.fr/kpng/pkg/server/watchstate"
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
