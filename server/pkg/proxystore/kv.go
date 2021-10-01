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
