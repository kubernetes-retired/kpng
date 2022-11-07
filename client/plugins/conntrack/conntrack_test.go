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

package conntrack

import (
	"context"
	"flag"
	"fmt"

	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	exectesting "k8s.io/utils/exec/testing"

	api "sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

func ExampleConntrack() {
	// setup
	klog.InitFlags(nil)
	flag.Set("v", "4")
	execer = printCmdsExecer{}

	ct := New()

	// initial state
	state := []*fullstate.ServiceEndpoints{
		{
			Service: &api.Service{
				Namespace: "test-ns",
				Name:      "test-svc",
				Type:      "ClusterIP",
				IPs: &api.ServiceIPs{
					ClusterIPs: api.NewIPSet("10.1.1.1"),
				},
				Ports: []*api.PortMapping{
					{
						Name:       "p1",
						Protocol:   api.Protocol_TCP,
						Port:       80,
						TargetPort: 8080,
					},
					{
						Name:       "p2",
						Protocol:   api.Protocol_UDP,
						Port:       53,
						TargetPort: 5353,
					},
				},
			},
			Endpoints: []*api.Endpoint{
				{
					IPs: api.NewIPSet("10.1.2.1"),
				},
			},
		},
	}

	fmt.Println("-- initial state --")
	ct.Callback(arrayCh(state))

	fmt.Println("-- add one endpoint --")
	state[0].Endpoints = append(state[0].Endpoints, &api.Endpoint{IPs: api.NewIPSet("10.1.3.1")})
	ct.Callback(arrayCh(state))

	fmt.Println("-- remove one endpoint --")
	state[0].Endpoints = state[0].Endpoints[:1]
	ct.Callback(arrayCh(state))

	fmt.Println("-- remove one service --")
	state = state[:0]
	ct.Callback(arrayCh(state))

	// Output:
	// -- initial state --
	// /bin/conntrack [-D -p tcp --dport 80 --orig-dst 10.1.1.1]
	// /bin/conntrack [-D -p udp --dport 53 --orig-dst 10.1.1.1]
	// -- add one endpoint --
	// -- remove one endpoint --
	// /bin/conntrack [-D -p udp --dport 53 --dst-nat 10.1.3.1 --orig-dst 10.1.1.1]
	// -- remove one service --
	// /bin/conntrack [-D -p udp --dport 53 --dst-nat 10.1.2.1 --orig-dst 10.1.1.1]

}

func arrayCh[T any](ts []T) <-chan T {
	ch := make(chan T, 1)
	go func() {
		for _, t := range ts {
			ch <- t
		}
		close(ch)
	}()
	return ch
}

type printCmdsExecer struct{}

var _ exec.Interface = printCmdsExecer{}

func (e printCmdsExecer) Command(cmd string, args ...string) exec.Cmd {
	fmt.Println(cmd, args)
	return exectesting.InitFakeCmd(&exectesting.FakeCmd{
		CombinedOutputScript: []exectesting.FakeAction{
			func() ([]byte, []byte, error) {
				return []byte{}, []byte{}, nil
			},
		},
	}, cmd, args...)
}

func (e printCmdsExecer) CommandContext(ctx context.Context, cmd string, args ...string) exec.Cmd {
	return e.Command(cmd, args...)
}

func (e printCmdsExecer) LookPath(file string) (string, error) {
	return "/bin/" + file, nil
}
