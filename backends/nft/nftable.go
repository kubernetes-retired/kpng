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
	"sigs.k8s.io/kpng/client/diffstore2"
)

var (
	table4 = newNftable("ip", "k8s_svc")
	table6 = newNftable("ip6", "k8s_svc6")

    allTables = []*nftable{table4, table6}
)

type Leaf  = diffstore2.BufferLeaf
type Item  = diffstore2.Item[string, *Leaf]
type Store = diffstore2.Store[string, *Leaf]

func newNftable(family, name string) *nftable {
    return &nftable{
        Family: family,
        Name:   name,
        Chains: diffstore2.NewBufferStore[string](),
        Maps:   diffstore2.NewBufferStore[string](),
    }
}

type nftable struct {
	Family string
	Name   string
	Chains *Store
	Maps   *Store
}

func (n *nftable) Reset() {
    n.Chains.Reset()
    n.Maps.Reset()
}

func (n *nftable) RunDeferred() {
    n.Chains.RunDeferred()
    n.Maps.RunDeferred()
}

func (n *nftable) Done() {
    n.Chains.Done()
    n.Maps.Done()
}

type KindStore struct {
    Kind string
    Store *Store
}

func (n *nftable) KindStores() []KindStore {
    return []KindStore{
        { "map", n.Maps },
        { "chain", n.Chains },
    }
}

func (n *nftable) Changed() bool {
    return n.Chains.HasChanges() || n.Maps.HasChanges()
}
