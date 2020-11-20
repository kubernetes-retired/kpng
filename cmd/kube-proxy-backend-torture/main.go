package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/cespare/xxhash"
	"github.com/gogo/protobuf/proto"
	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/endpoints"
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

	localnetv1.RegisterEndpointsService(srv, localnetv1.NewEndpointsService(localnetv1.UnstableEndpointsService(
		&serverendpoints.Server{
			Correlator: tortureCorrelator{},
		})))

	lis := server.MustListen(*bindSpec)
	srv.Serve(lis)
}

type tortureCorrelator struct{}

var _ serverendpoints.Correlator = tortureCorrelator{}

func (_ tortureCorrelator) NextKVs(lastKnownRev uint64) (results []endpoints.KV, rev uint64) {
	time.Sleep(*sleepFlag)

	args := flag.Args()

	if int(lastKnownRev) >= len(args) {
		log.Print("waiting forever (rev: ", lastKnownRev, ")")
		select {} // tests finished, sleep forever
	}

	spec := args[lastKnownRev]

	var nSvc, nEpPerSvc int

	_, err := fmt.Sscanf(spec, "%d:%d", &nSvc, &nEpPerSvc)
	if err != nil {
		log.Fatal("failed to parse arg: ", spec, ": ", err)
	}

	rev = lastKnownRev
	log.Print("sending spec ", spec, " (rev: ", rev, ")")

	svcIP := ipGen(net.ParseIP("10.0.0.0"))
	epIP := ipGen(net.ParseIP("10.128.0.0"))

	pb := proto.NewBuffer(nil)

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

		seps := &localnetv1.ServiceEndpoints{
			Service:   svc,
			Endpoints: make([]*localnetv1.Endpoint, nEpPerSvc),
		}

		for e := 0; e < nEpPerSvc; e++ {
			ep := &localnetv1.Endpoint{
				Conditions: &localnetv1.EndpointConditions{
					Local:    true,
					Ready:    true,
					Selected: true,
				},
			}
			ep.AddAddress(epIP.Next().String())
			seps.Endpoints[e] = ep
		}

		pb.Reset()
		pb.Marshal(seps)

		results = append(results, endpoints.KV{
			Namespace:     svc.Namespace,
			Name:          svc.Name,
			EndpointsHash: xxhash.Sum64(pb.Bytes()),
			Endpoints:     seps,
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
