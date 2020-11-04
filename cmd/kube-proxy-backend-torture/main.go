package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/server"
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

	localnetv1.RegisterEndpointsService(srv, localnetv1.NewEndpointsService(localnetv1.UnstableEndpointsService(endpoints{})))

	lis := server.MustListen(*bindSpec)
	srv.Serve(lis)
}

type endpoints struct{}

func (_ endpoints) Next(filter *localnetv1.NextFilter, res localnetv1.Endpoints_NextServer) (err error) {
	time.Sleep(*sleepFlag)

	if filter.InstanceID != instanceID {
		filter.Rev = 0
	}

	args := flag.Args()

	if int(filter.Rev) >= len(args) {
		select {} // tests finished, sleep forever
	}

	spec := args[filter.Rev]

	var nSvc, nEpPerSvc int

	_, err = fmt.Sscanf(spec, "%d:%d", &nSvc, &nEpPerSvc)
	if err != nil {
		return
	}

	res.Send(&localnetv1.NextItem{
		Next: &localnetv1.NextFilter{
			InstanceID: instanceID,
			Rev:        filter.Rev + 1,
		},
	})

	svcIP := ipGen(net.ParseIP("10.0.0.0"))
	epIP := ipGen(net.ParseIP("10.128.0.0"))

	for s := 0; s < nSvc; s++ {
		seps := &localnetv1.ServiceEndpoints{
			Namespace: "default",
			Name:      fmt.Sprintf("svc-%d", s),
			Type:      "ClusterIP",
			IPs: &localnetv1.ServiceIPs{
				ClusterIP:   svcIP.Next().String(),
				ExternalIPs: &localnetv1.IPSet{},
			},
			Endpoints: make([]*localnetv1.Endpoint, nEpPerSvc),
			Ports: []*localnetv1.PortMapping{
				{
					Protocol:   localnetv1.Protocol_TCP,
					Port:       80,
					TargetPort: 8080,
				},
			},
		}

		for e := 0; e < nEpPerSvc; e++ {
			ep := &localnetv1.Endpoint{}
			ep.AddAddress(epIP.Next().String())
			seps.Endpoints[e] = ep
		}

		res.Send(&localnetv1.NextItem{
			Endpoints: seps,
		})
	}

	return
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
