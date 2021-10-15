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
	"strconv"
	"time"

	"github.com/cespare/xxhash"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"k8s.io/klog"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/pkg/diffstore"
	"sigs.k8s.io/kpng/server/pkg/server"
	"sigs.k8s.io/kpng/server/pkg/server/watchstate"
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

	localnetv1.RegisterEndpointsService(srv, localnetv1.NewEndpointsService(localnetv1.UnstableEndpointsService(watchSrv{})))

	lis := server.MustListen(*bindSpec)
	srv.Serve(lis)
}

var syncItem = &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{}}

type watchSrv struct{}

func (s watchSrv) Watch(res localnetv1.Endpoints_WatchServer) error {
	w := watchstate.New(res, []localnetv1.Set{localnetv1.Set_ServicesSet, localnetv1.Set_EndpointsSet})

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
		w.SendUpdates(localnetv1.Set_ServicesSet)
		w.SendUpdates(localnetv1.Set_EndpointsSet)
		w.SendDeletes(localnetv1.Set_EndpointsSet)
		w.SendDeletes(localnetv1.Set_ServicesSet)

		w.Reset(diffstore.ItemDeleted)

		// change set sent
		w.SendSync()

		if w.Err != nil {
			return w.Err
		}
	}
}

func injectState(rev uint64, w *watchstate.WatchState) {
	time.Sleep(*sleepFlag)

	svcs := w.StoreFor(localnetv1.Set_ServicesSet)
	seps := w.StoreFor(localnetv1.Set_EndpointsSet)

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
		svc := &localnetv1.Service{
			Namespace: "default",
			Name:      fmt.Sprintf("svc-%d", s),
			Type:      "ClusterIP",
			IPs: &localnetv1.ServiceIPs{
				ClusterIPs:  localnetv1.NewIPSet(svcIP.Next().String()),
				ExternalIPs: &localnetv1.IPSet{},
			},
			Ports: []*localnetv1.PortMapping{
				{
					Protocol:   localnetv1.Protocol_TCP,
					Port:       80,
					TargetPort: 8080,
				},
			},
		}

		svcs.Set([]byte(svc.Namespace+"/"+svc.Name), hashOf(svc), svc)

		for e := 0; e < nEpPerSvc; e++ {
			ep := &localnetv1.Endpoint{}
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
