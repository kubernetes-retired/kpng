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

package nft

import (
	"sort"
	"strings"

	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/diffstore"
)

var (
	table4 = newNftable("ip", "k8s_svc")
	table6 = newNftable("ip6", "k8s_svc6")

	allTables = []*nftable{table4, table6}
)

type Leaf = diffstore.BufferLeaf
type Item = diffstore.Item[string, *Leaf]
type Store = diffstore.Store[string, *Leaf]

func newNftable(family, name string) *nftable {
	return &nftable{
		Family: family,
		Name:   name,
		Chains: diffstore.NewBufferStore[string](),
		Maps:   diffstore.NewBufferStore[string](),
		Sets:   diffstore.NewBufferStore[string](),
	}
}

type nftable struct {
	Family string
	Name   string
	Chains *Store
	Maps   *Store
	Sets   *Store
}

func (n *nftable) nftIPType() string {
	switch n.Family {
	case "ip":
		return "ipv4_addr"
	case "ip6":
		return "ipv6_addr"
	default:
		panic("unknown family: " + n.Family)
	}
}

func (n *nftable) IPsFromSet(set *localv1.IPSet) []string {
	switch n.Family {
	case "ip":
		return set.V4
	case "ip6":
		return set.V6
	default:
		return nil
	}
}

func (n *nftable) Reset() {
	n.Chains.Reset()
	n.Maps.Reset()
	n.Sets.Reset()
}

func (n *nftable) RunDeferred() {
	n.Chains.RunDeferred()
	n.Maps.RunDeferred()
	n.Sets.RunDeferred()
}

func (n *nftable) Done() {
	n.Chains.Done()
	n.Maps.Done()
	n.Sets.Done()
}

type KindStore struct {
	Kind  string
	Store *Store
}

func (n *nftable) KindStores() []KindStore {
	return []KindStore{
		{"map", n.Maps},
		{"set", n.Sets},
		{"chain", n.Chains},
	}
}

func (n *nftable) Changed() bool {
	return n.Chains.HasChanges() || n.Maps.HasChanges()
}

type KindItem struct {
	Kind string
	Item *Item
}

func (ki KindItem) Prio() int {
	key := ki.Item.Key()

	kparts := strings.Split(key, "_")

	switch {
	case ki.Kind == "chain" && len(kparts) == 5 && kparts[3] == "ep":
		// endpoint chains
		// chain svc_default_kubernetes_ep_ac120002
		return 0
	case ki.Kind == "map" && len(kparts) == 4 && kparts[3] == "eps":
		// per service endpoints dispatch
		// map svc_services-4569_affinity-nodeport-transition_eps
		return 1
	case ki.Kind == "map" && len(kparts) == 5 && kparts[3] == "eps":
		// per service endpoints dispatch (named ports)
		// map svc_services-4569_affinity-nodeport-transition_eps_named-port
		return 2

	case kparts[0] == "svc":
		// service-related
		// chain svc_default_kubernetes_dnat
		return 20

	default:
		return 100
	}
}

// OrderedChanges return the changes in an order that should not break nft (dependencies first)
func (n *nftable) OrderedChanges(all bool) (elements []KindItem) {
	// collect everything
	elements = make([]KindItem, 0)

	for _, ks := range n.KindStores() {
		var items []*Item
		if all {
			items = ks.Store.List()
		} else {
			items = ks.Store.Changed()
		}

		for _, item := range items {
			elements = append(elements, KindItem{
				Kind: ks.Kind,
				Item: item,
			})
		}
	}

	// sort
	sort.Slice(elements, func(i, j int) bool {
		e1, e2 := elements[i], elements[j]
		p1, p2 := e1.Prio(), e2.Prio()
		if p1 == p2 {
			return e1.Item.Key() < e2.Item.Key()
		}
		return p1 < p2
	})

	return
}
