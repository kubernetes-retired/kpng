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
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/lightdiffstore"
	"sigs.k8s.io/kpng/server/pkg/server"
	"sigs.k8s.io/kpng/server/pkg/server/watchstate"
	"sigs.k8s.io/kpng/server/serde"
)

var (
	bindSpec  = flag.String("listen", "tcp://127.0.0.1:12090", "local API listen spec formatted as protocol://address")
	sleepFlag = flag.Duration("sleep", 1*time.Millisecond, "sleep before sending dataset")

	instanceID uint64
)

func main() {
	flag.Parse()
	fmt.Println("This is trivial change.")
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package with cryptographically secure random number generator")
	}
	instanceID = binary.LittleEndian.Uint64(b[:])

	srv := grpc.NewServer()

	localv1.RegisterSetsServer(srv, watchSrv{})

	lis := server.MustListen(*bindSpec)
	srv.Serve(lis)
}

var syncItem = &localv1.OpItem{Op: &localv1.OpItem_Sync{}}

type watchSrv struct {
	localv1.UnimplementedSetsServer
}

func (s watchSrv) Watch(res localv1.Sets_WatchServer) error {
	w := watchstate.New(res, []localv1.Set{localv1.Set_ServicesSet, localv1.Set_EndpointsSet})

	var i uint64
	for {
		// wait for client request
		req, err := res.Recv()
		if err != nil {
			return grpc.Errorf(codes.Aborted, "recv error: %v", err)
		}

		log.Print("got watch request: ", req)

		injectState(i, w)
		i++

		// send diff
		w.SendUpdates(localv1.Set_ServicesSet)
		w.SendUpdates(localv1.Set_EndpointsSet)
		w.SendDeletes(localv1.Set_EndpointsSet)
		w.SendDeletes(localv1.Set_ServicesSet)

		w.Reset(lightdiffstore.ItemDeleted)

		// change set sent
		w.SendSync()

		if w.Err != nil {
			return w.Err
		}
	}
}

func injectState(rev uint64, w *watchstate.WatchState) {
	time.Sleep(*sleepFlag)

	svcs := w.StoreFor(localv1.Set_ServicesSet)
	seps := w.StoreFor(localv1.Set_EndpointsSet)

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

	for s := 0; s < nSvc; s++ {
		svc := &localv1.Service{
			Namespace: "default",
			Name:      fmt.Sprintf("svc-%d", s),
			Type:      "ClusterIP",
			IPs: &localv1.ServiceIPs{
				ClusterIPs:  localv1.NewIPSet(svcIP.Next().String()),
				ExternalIPs: &localv1.IPSet{},
			},
			Ports: []*localv1.PortMapping{
				{
					Protocol:   localv1.Protocol_TCP,
					Port:       80,
					TargetPort: 8080,
				},
			},
		}

		svcs.Set([]byte(svc.Namespace+"/"+svc.Name), serde.Hash(svc), svc)

		for e := 0; e < nEpPerSvc; e++ {
			ep := &localv1.Endpoint{}
			ep.AddAddress(epIP.Next().String())

			h := serde.Hash(ep)
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
