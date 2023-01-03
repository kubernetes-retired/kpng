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

package proxystore

import (
	"strings"

	"github.com/google/btree"

	"sigs.k8s.io/kpng/api/globalv1"
)

// BTreeItem is the item that KPNG stores in the underlying BTree representation is used to sort, iterate, and lookup
// objects from the overall global data model.
type BTreeItem struct {
	// TODO: What is this for?
	Sync *bool

	// The Metadata about the Btree item that we use to sort and/or lookup...
	Set       Set
	Namespace string
	Name      string
	Source    string
	Key       string
	Value     Hashed

	// The BTreeItem'proxyStore underlying information content...
	Service  *globalv1.ServiceInfo
	Endpoint *globalv1.EndpointInfo
	Node     *globalv1.NodeInfo
}

// GetPath returns the fully qualified
func (a *BTreeItem) GetPath() string {
	return strings.Join([]string{a.Namespace, a.Name, a.Source, a.Key}, "|")
}

func (a *BTreeItem) SetFromPath(path string) {
	p := strings.Split(path, "|")
	a.Namespace, a.Name, a.Source, a.Key = p[0], p[1], p[2], p[3]
}

// Less implements the Btree Item interface, such that the Btree can hold all (service, endpoint, whatever) objects that the proxystore needs to find, insert, update, or delete efficiently (https://pkg.go.dev/github.com/google/btree#Item).
// The items are n-sorted via set, namespace, name, and source.  Thus, the items type (i.e. Endpoint , Service, ...) is
// the first category for sorting, and the  least significant category of sorting is the Source of the item.
func (a *BTreeItem) Less(i btree.Item) bool {
	b := i.(*BTreeItem)

	if a.Set != b.Set {
		return a.Set < b.Set
	}
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}

	// TODO we should add a comment about when this field, actually matters ?  I dont think we use it for any KPNG logic,
	// but i do see it read/copied around alot.
	if a.Source != b.Source {
		return a.Source < b.Source
	}

	return a.Key < b.Key
}
