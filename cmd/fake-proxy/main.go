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

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/cespare/xxhash"
	"github.com/gogo/protobuf/proto"
	"gopkg.in/yaml.v2"
	"k8s.io/klog"

	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
	"m.cluseau.fr/kube-proxy2/pkg/diffstore"
	"m.cluseau.fr/kube-proxy2/pkg/proxy"
	"m.cluseau.fr/kube-proxy2/pkg/proxystore"
	"m.cluseau.fr/kube-proxy2/pkg/server"
	"m.cluseau.fr/kube-proxy2/pkg/server/endpoints"
	"m.cluseau.fr/kube-proxy2/pkg/server/global"
	"m.cluseau.fr/kube-proxy2/pkg/server/watchstate"
)

func main() {
	bindSpec := flag.String("listen", "tcp://127.0.0.1:12090", "local API listen spec formatted as protocol://address")
	configPath := flag.String("config", "config.yaml", "proxy data to serve")
	flag.Parse()

	srv, err := proxy.NewServer()

	go pollConfig(*configPath, srv.Store)

	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	// setup correlator
	localnetv1.RegisterEndpointsService(srv.GRPC, localnetv1.NewEndpointsService(localnetv1.UnstableEndpointsService(&endpoints.Server{
		Store: srv.Store,
	})))

	localnetv1.RegisterGlobalService(srv.GRPC, localnetv1.NewGlobalService(localnetv1.UnstableGlobalService(&global.Server{
		Store: srv.Store,
	})))

	// handle exit signals
	go func() {
		proxy.WaitForTermSignal()
		srv.Stop()
	}()

	lis := server.MustListen(*bindSpec)
	err = srv.GRPC.Serve(lis)
	if err != nil {
		klog.Fatal(err)
	}
}

type Config struct {
	Nodes    []*localnetv1.Node
	Services []ServiceAndEndpoints
}

type ServiceAndEndpoints struct {
	Service      *localnetv1.Service
	TopologyKeys []string
	Endpoints    []*localnetv1.EndpointInfo
}

func pollConfig(configPath string, store *proxystore.Store) {
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

		config := &Config{}
		err = yaml.UnmarshalStrict(configBytes, config)
		if err != nil {
			log.Print("failed to parse config: ", err)
			continue
		}

		log.Print(config)

		diffNodes := w.StoreFor(proxystore.Nodes)
		diffSvcs := w.StoreFor(proxystore.Services)
		diffEPs := w.StoreFor(proxystore.Endpoints)

		for _, node := range config.Nodes {
			diffNodes.Set([]byte(node.Name), hashOf(node), node)
		}

		for _, se := range config.Services {
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
