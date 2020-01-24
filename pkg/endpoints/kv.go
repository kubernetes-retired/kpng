package endpoints

import (
	"github.com/google/btree"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

type endpointsKV struct {
	Namespace string
	Name      string

	Endpoints     *localnetv1.ServiceEndpoints
	EndpointsHash uint64
}

var _ btree.Item = endpointsKV{}

func (kv endpointsKV) Less(i btree.Item) bool {
	kv2 := i.(endpointsKV)

	if kv.Namespace != kv2.Namespace {
		return kv.Namespace < kv2.Namespace
	}

	return kv.Name < kv2.Name
}
