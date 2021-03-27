package file2store

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/cespare/xxhash"
	"github.com/gogo/protobuf/proto"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/kpng/jobs/store2file"
	"sigs.k8s.io/kpng/pkg/api/localnetv1"
	"sigs.k8s.io/kpng/pkg/diffstore"
	"sigs.k8s.io/kpng/pkg/proxystore"
	"sigs.k8s.io/kpng/pkg/server/watchstate"
)

type Job struct {
	FilePath string
	Store    *proxystore.Store
}

func (j *Job) Run(ctx context.Context) {
	configPath := j.FilePath
	store := j.Store

	w := watchstate.New(nil, proxystore.AllSets)

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
			log.Print("failed to stat config: ", err)
			continue
		}

		if !stat.ModTime().After(mtime) {
			continue
		}

		mtime = stat.ModTime()

		configBytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			log.Print("failed to read config: ", err)
			continue
		}

		state := &store2file.GlobalState{}
		err = yaml.UnmarshalStrict(configBytes, state)
		if err != nil {
			log.Print("failed to parse config: ", err)
			continue
		}

		diffNodes := w.StoreFor(proxystore.Nodes)
		diffSvcs := w.StoreFor(proxystore.Services)
		diffEPs := w.StoreFor(proxystore.Endpoints)

		for _, node := range state.Nodes {
			diffNodes.Set([]byte(node.Name), hashOf(node), node)
		}

		for _, se := range state.Services {
			svc := se.Service

			if svc.Namespace == "" {
				svc.Namespace = "default"
			}

			si := &localnetv1.ServiceInfo{
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
						ep.Conditions = &localnetv1.EndpointConditions{Ready: true}
					}

					ba, _ := proto.Marshal(ep)
					h.Write(ba)
				}

				diffEPs.Set(fullName, h.Sum64(), se.Endpoints)
			}
		}

		store.Update(func(tx *proxystore.Tx) {
			for _, u := range diffNodes.Updated() {
				log.Print("U node ", string(u.Key))
				tx.SetNode(u.Value.(*localnetv1.Node))
			}
			for _, u := range diffSvcs.Updated() {
				log.Print("U service ", string(u.Key))
				si := u.Value.(*localnetv1.ServiceInfo)
				tx.SetService(si.Service, si.TopologyKeys)
			}
			for _, u := range diffEPs.Updated() {
				log.Print("U endpoints ", string(u.Key))
				key := string(u.Key)
				eis := u.Value.([]*localnetv1.EndpointInfo)

				tx.SetEndpointsOfSource(path.Dir(key), path.Base(key), eis)
			}

			for _, d := range diffEPs.Deleted() {
				log.Print("D endpoints ", string(d.Key))
				key := string(d.Key)
				tx.DelEndpointsOfSource(path.Dir(key), path.Base(key))
			}
			for _, d := range diffSvcs.Deleted() {
				log.Print("D service ", string(d.Key))
				key := string(d.Key)
				tx.DelService(path.Dir(key), path.Base(key))
			}
			for _, d := range diffNodes.Deleted() {
				log.Print("D node ", string(d.Key))
				tx.DelNode(string(d.Key))
			}

			for _, set := range proxystore.AllSets {
				tx.SetSync(set)
			}
		})

		for _, set := range proxystore.AllSets {
			w.StoreFor(set).Reset(diffstore.ItemDeleted)
		}
	}
}
