package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/server"
	serverendpoints "github.com/mcluseau/kube-proxy2/pkg/server/endpoints"
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
	w := serverendpoints.NewWatchState(res)

	var i uint64
	for {
		// wait for client request
		_, err := res.Recv()
		if err != nil {
			return grpc.Errorf(codes.Aborted, "recv error: %v", err)
		}

		injectState(i, w)
		i++

		w.SendDiff()

		// change set sent
		w.Send(syncItem)

		if w.Err != nil {
			return w.Err
		}
	}
}

func injectState(rev uint64, w *serverendpoints.WatchState) {
	time.Sleep(*sleepFlag)

	args := flag.Args()

	if int(rev) >= len(args) {
		log.Print("waiting forever (rev: ", rev, ")")
		select {} // tests finished, sleep forever
	}

	spec := args[rev]

	var nSvc, nEpPerSvc int

	_, err := fmt.Sscanf(spec, "%d:%d", &nSvc, &nEpPerSvc)
	if err != nil {
		log.Fatal("failed to parse arg: ", spec, ": ", err)
	}

	log.Print("sending spec ", spec, " (rev: ", rev, ")")

	svcIP := ipGen(net.ParseIP("10.0.0.0"))
	epIP := ipGen(net.ParseIP("10.128.0.0"))

	for s := 0; s < nSvc; s++ {
		svc := &localnetv1.Service{
			Namespace: "default",
			Name:      fmt.Sprintf("svc-%d", s),
			Type:      "ClusterIP",
			IPs: &localnetv1.ServiceIPs{
				ClusterIP:   svcIP.Next().String(),
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

		w.Svcs.Set([]byte(svc.Namespace+"/"+svc.Name), w.HashOf(svc), svc)

		for e := 0; e < nEpPerSvc; e++ {
			ep := &localnetv1.Endpoint{}
			ep.AddAddress(epIP.Next().String())
			h := w.HashOf(ep)
			w.Seps.Set([]byte(svc.Namespace+"/"+svc.Name+"/"+strconv.FormatUint(h, 16)), h, ep)
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
