package file2store

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/cespare/xxhash"
	"github.com/gogo/protobuf/proto"
	"gopkg.in/yaml.v2"
	"k8s.io/klog"

	localnetv12 "sigs.k8s.io/kpng/api/localnetv1"
	store2file2 "sigs.k8s.io/kpng/server/jobs/store2file"
	diffstore2 "sigs.k8s.io/kpng/server/pkg/diffstore"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
	watchstate2 "sigs.k8s.io/kpng/server/pkg/server/watchstate"
)

type Job struct {
	FilePath string
	Store    *proxystore2.Store
}

func (j *Job) Run(ctx context.Context) {
	configPath := j.FilePath
	store := j.Store

	w := watchstate2.New(nil, proxystore2.AllSets)

	pb := proto.NewBuffer(make([]byte, 0))
	hashOf := func(m proto.Message) uint64 {
		defer pb.Reset()

		err := pb.Marshal(m)
		if err != nil {
			panic(err)
		}

		h := xxhash.Sum64(pb.Bytes())
		return h
	}

	mtime := time.Time{}

	for range time.Tick(time.Second) {
		if ctx.Err() != nil {
			return
		}

		stat, err := os.Stat(configPath)
		if err != nil {
			klog.Info("failed to stat config: ", err)
			continue
		}

		if !stat.ModTime().After(mtime) {
			continue
		}

		mtime = stat.ModTime()

		configBytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			klog.Info("failed to read config: ", err)
			continue
		}

		state := &store2file2.GlobalState{}
		err = yaml.UnmarshalStrict(configBytes, state)
		if err != nil {
			klog.Info("failed to parse config: ", err)
			continue
		}

		diffNodes := w.StoreFor(proxystore2.Nodes)
		diffSvcs := w.StoreFor(proxystore2.Services)
		diffEPs := w.StoreFor(proxystore2.Endpoints)

		for _, node := range state.Nodes {
			diffNodes.Set([]byte(node.Name), hashOf(node), node)
		}

		for _, se := range state.Services {
			svc := se.Service

			if svc.Namespace == "" {
				svc.Namespace = "default"
			}

			si := &localnetv12.ServiceInfo{
				Service:      se.Service,
				TopologyKeys: se.TopologyKeys,
			}

			fullName := []byte(svc.Namespace + "/" + svc.Name)

			diffSvcs.Set(fullName, hashOf(si), si)

			if len(se.Endpoints) != 0 {
				h := xxhash.New()
				for _, ep := range se.Endpoints {
					ep.Namespace = svc.Namespace
					ep.SourceName = svc.Name
					ep.ServiceName = svc.Name

					if ep.Conditions == nil {
						ep.Conditions = &localnetv12.EndpointConditions{Ready: true}
					}

					ba, _ := proto.Marshal(ep)
					h.Write(ba)
				}

				diffEPs.Set(fullName, h.Sum64(), se.Endpoints)
			}
		}

		store.Update(func(tx *proxystore2.Tx) {
			for _, u := range diffNodes.Updated() {
				klog.Info("U node ", string(u.Key))
				tx.SetNode(u.Value.(*localnetv12.Node))
			}
			for _, u := range diffSvcs.Updated() {
				klog.Info("U service ", string(u.Key))
				si := u.Value.(*localnetv12.ServiceInfo)
				tx.SetService(si.Service, si.TopologyKeys)
			}
			for _, u := range diffEPs.Updated() {
				klog.Info("U endpoints ", string(u.Key))
				key := string(u.Key)
				eis := u.Value.([]*localnetv12.EndpointInfo)

				tx.SetEndpointsOfSource(path.Dir(key), path.Base(key), eis)
			}

			for _, d := range diffEPs.Deleted() {
				klog.Info("D endpoints ", string(d.Key))
				key := string(d.Key)
				tx.DelEndpointsOfSource(path.Dir(key), path.Base(key))
			}
			for _, d := range diffSvcs.Deleted() {
				klog.Info("D service ", string(d.Key))
				key := string(d.Key)
				tx.DelService(path.Dir(key), path.Base(key))
			}
			for _, d := range diffNodes.Deleted() {
				klog.Info("D node ", string(d.Key))
				tx.DelNode(string(d.Key))
			}

			for _, set := range proxystore2.AllSets {
				tx.SetSync(set)
			}
		})

		for _, set := range proxystore2.AllSets {
			w.StoreFor(set).Reset(diffstore2.ItemDeleted)
		}
	}
}
