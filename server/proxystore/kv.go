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

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

type KV struct {
	Sync      *bool
	Set       Set
	Namespace string
	Name      string
	Source    string
	Key       string

	Value Hashed

	Service  *localnetv1.ServiceInfo
	Endpoint *localnetv1.EndpointInfo
	Node     *localnetv1.NodeInfo
}

func (a *KV) Path() string {
	return strings.Join([]string{a.Namespace, a.Name, a.Source, a.Key}, "|")
}

func (a *KV) SetPath(path string) {
	p := strings.Split(path, "|")
	a.Namespace, a.Name, a.Source, a.Key = p[0], p[1], p[2], p[3]
}

func (a *KV) Less(i btree.Item) bool {
	b := i.(*KV)

	if a.Set != b.Set {
		return a.Set < b.Set
	}
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	if a.Source != b.Source {
		return a.Source < b.Source
	}

	return a.Key < b.Key
}
