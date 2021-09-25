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
	"fmt"
	"math/rand"
	"net"
	localnetv12 "sigs.k8s.io/kpng/server/pkg/api/localnetv1"
	diffstore2 "sigs.k8s.io/kpng/server/pkg/diffstore"
	server2 "sigs.k8s.io/kpng/server/pkg/server"
	watchstate2 "sigs.k8s.io/kpng/server/pkg/server/watchstate"
	"strconv"
	"time"

	"github.com/cespare/xxhash"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"k8s.io/klog"
)

var (
	bindSpec  = flag.String("listen", "tcp://127.0.0.1:12090", "local API listen spec formatted as protocol://address")
	sleepFlag = flag.Duration("sleep", 1*time.Millisecond, "sleep before sending dataset")

	instanceID uint64
)

func main() {
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	instanceID = rand.Uint64()

	srv := grpc.NewServer()

	localnetv12.RegisterEndpointsService(srv, localnetv12.NewEndpointsService(localnetv12.UnstableEndpointsService(watchSrv{})))

	lis := server2.MustListen(*bindSpec)
	srv.Serve(lis)
}

var syncItem = &localnetv12.OpItem{Op: &localnetv12.OpItem_Sync{}}

type watchSrv struct{}

func (s watchSrv) Watch(res localnetv12.Endpoints_WatchServer) error {
	w := watchstate2.New(res, []localnetv12.Set{localnetv12.Set_ServicesSet, localnetv12.Set_EndpointsSet})

	var i uint64
	for {
		// wait for client request
		_, err := res.Recv()
		if err != nil {
			return grpc.Errorf(codes.Aborted, "recv error: %v", err)
		}

		injectState(i, w)
		i++

		// send diff
		w.SendUpdates(localnetv12.Set_ServicesSet)
		w.SendUpdates(localnetv12.Set_EndpointsSet)
		w.SendDeletes(localnetv12.Set_EndpointsSet)
		w.SendDeletes(localnetv12.Set_ServicesSet)

		w.Reset(diffstore2.ItemDeleted)

		// change set sent
		w.SendSync()

		if w.Err != nil {
			return w.Err
		}
	}
}

func injectState(rev uint64, w *watchstate2.WatchState) {
	time.Sleep(*sleepFlag)

	svcs := w.StoreFor(localnetv12.Set_ServicesSet)
	seps := w.StoreFor(localnetv12.Set_EndpointsSet)

	args := flag.Args()

	if int(rev) >= len(args) {
		klog.Info("waiting forever (rev: ", rev, ")")
		select {} // tests finished, sleep forever
	}

	spec := args[rev]

	var nSvc, nEpPerSvc int

	_, err := fmt.Sscanf(spec, "%d:%d", &nSvc, &nEpPerSvc)
	if err != nil {
		klog.Fatal("failed to parse arg: ", spec, ": ", err)
	}

	klog.Info("sending spec ", spec, " (rev: ", rev, ")")

	svcIP := ipGen(net.ParseIP("10.0.0.0"))
	epIP := ipGen(net.ParseIP("10.128.0.0"))

	pb := proto.NewBuffer(make([]byte, 0))
	hashOf := func(m proto.Message) uint64 {
		pb.Marshal(m)
		h := xxhash.Sum64(pb.Bytes())
		pb.Reset()
		return h
	}

	for s := 0; s < nSvc; s++ {
		svc := &localnetv12.Service{
			Namespace: "default",
			Name:      fmt.Sprintf("svc-%d", s),
			Type:      "ClusterIP",
			IPs: &localnetv12.ServiceIPs{
				ClusterIPs:  localnetv12.NewIPSet(svcIP.Next().String()),
				ExternalIPs: &localnetv12.IPSet{},
			},
			Ports: []*localnetv12.PortMapping{
				{
					Protocol:   localnetv12.Protocol_TCP,
					Port:       80,
					TargetPort: 8080,
				},
			},
		}

		svcs.Set([]byte(svc.Namespace+"/"+svc.Name), hashOf(svc), svc)

		for e := 0; e < nEpPerSvc; e++ {
			ep := &localnetv12.Endpoint{}
			ep.AddAddress(epIP.Next().String())

			h := hashOf(ep)
			seps.Set([]byte(svc.Namespace+"/"+svc.Name+"/"+strconv.FormatUint(h, 16)), h, ep)
		}
	}
}

type ipGen net.IP

func (ip ipGen) Next() net.IP {
	for i := len(ip) - 1; i != -1; i-- {
		if ip[i] == 0xff {
			ip[i] = 0
			continue
		}

		ip[i]++

		next := make([]byte, len(ip))
		copy(next, ip)
		return net.IP(next)
	}

	panic("no more IPs!")
}
