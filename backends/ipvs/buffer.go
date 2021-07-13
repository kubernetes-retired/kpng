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

package ipvs

import (
	"github.com/OneOfOne/xxhash"
	"github.com/google/btree"
)

var (
	ipvsTable4 = &ipvsTable{"ip", btree.New(4)}
	ipvsTable6 = &ipvsTable{"ip6", btree.New(4)}
)

type ipvsTable struct {
	Family string
	data   *btree.BTree
}
type NodeType int

const (
	virtualService NodeType = iota
	realServer
)

type virtualServiceInfo struct {
	protocol         string
	serviceIP        string
	servicePort      int32
	schedulingMethod string
}

type realServerInfo struct {
	endPointIP string
	targetPort int32
}
type buffer struct {
	kind           string
	name           string
	nodeType       NodeType
	previousHash   uint64
	currentHash    *xxhash.XXHash64
	virtualService virtualServiceInfo
	realServer     realServerInfo
}

var (
	_ btree.Item = &buffer{}
)

func (c *buffer) Less(i btree.Item) bool {
	return c.name < i.(*buffer).name
}

func (c *buffer) Changed() bool {
	if c.currentHash == nil {
		return c.previousHash != 0
	}
	return c.currentHash.Sum64() != c.previousHash
}

func (c *buffer) Created() bool {
	return c.previousHash == 0 && c.currentHash != nil
}

func (c *buffer) WriteVirtualServiceInfo(proto, svcIP, sched string, port int32) {
	if c.currentHash == nil {
		c.currentHash = xxhash.New64()
	}
	c.virtualService = virtualServiceInfo{protocol: proto, serviceIP: svcIP, servicePort: port, schedulingMethod: sched}
}

func (c *buffer) WriteRealServerInfo(endPointIP string, tPort int32, proto, svcIP string, svcPort int32) {
	if c.currentHash == nil {
		c.currentHash = xxhash.New64()
	}
	c.realServer = realServerInfo{endPointIP: endPointIP, targetPort: tPort}
	c.virtualService = virtualServiceInfo{protocol: proto, serviceIP: svcIP, servicePort: svcPort}
}

func (set *ipvsTable) Reset() {
	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*buffer)
		if cb.currentHash == nil {
			// no writes -> empty
			cb.previousHash = 0
		} else {
			cb.previousHash = cb.currentHash.Sum64()
			cb.currentHash = nil
		}
		return true
	})
}

func (set *ipvsTable) Get(nodeType NodeType, kind, name string) *buffer {
	i := set.data.Get(&buffer{name: name})

	if i == nil {
		if kind == "" {
			panic("can't create without kind")
		}

		i = &buffer{
			kind:     kind,
			name:     name,
			nodeType: nodeType,
		}
		set.data.ReplaceOrInsert(i)
	}

	cb := i.(*buffer)

	if kind != "" && kind != cb.kind {
		panic("wrong kind for " + name + ": " + kind + " (got " + cb.kind + ")")
	}

	return cb
}

func (set *ipvsTable) ListOfVirtualService() (virtSvcList []string) {
	virtSvcList = make([]string, 0, set.data.Len())

	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*buffer)
		if cb.nodeType == virtualService {
			if cb.currentHash != nil {
				virtSvcList = append(virtSvcList, cb.name)
			}
		}
		return true
	})
	return
}

func (set *ipvsTable) ListOfRealServer() (realSrvList []string) {
	realSrvList = make([]string, 0, set.data.Len())

	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*buffer)
		if cb.nodeType == realServer {
			if cb.currentHash != nil {
				realSrvList = append(realSrvList, cb.name)
			}
		}
		return true
	})

	return
}

func (set *ipvsTable) DeletedVirtualService() (virtSvcList []*buffer) {
	virtSvcList = make([]*buffer, 0)

	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*buffer)
		if cb.nodeType == virtualService {
			if cb.previousHash != 0 && cb.currentHash == nil {
				virtSvcList = append(virtSvcList, cb)
			}
		}
		return true
	})

	return
}

func (set *ipvsTable) DeletedRealServer() (realSrvList []*buffer) {
	realSrvList = make([]*buffer, 0)

	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*buffer)
		if cb.nodeType == realServer {
			if cb.previousHash != 0 && cb.currentHash == nil {
				realSrvList = append(realSrvList, cb)
			}
		}
		return true
	})

	return
}

func (set *ipvsTable) Changed() (changed bool) {
	changed = false

	set.data.Ascend(func(i btree.Item) bool {
		cb := i.(*buffer)
		if cb.Changed() {
			changed = true
		}
		return !changed
	})

	return
}
