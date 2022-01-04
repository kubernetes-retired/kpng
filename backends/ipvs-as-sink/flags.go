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

package ipvssink

import (
	"net"

	"github.com/spf13/pflag"
)

func (s *Backend) BindFlags(flags *pflag.FlagSet) {
	s.Config.BindFlags(flags)

	// real ipvs sink flags
	flags.BoolVar(&s.dryRun, "dry-run", false, "dry run (print instead of applying)")
	flags.StringSliceVar(&s.nodeAddresses, "node-address", interfaceAddresses(), "A comma-separated list of IPs to associate when using NodePort type. Defaults to all the Node addresses")
	flags.StringVar(&s.schedulingMethod, "scheduling-method", "rr", "Algorithm for allocating TCP conn & UDP datagrams to real servers. Values: rr,wrr,lc,wlc,lblc,lblcr,dh,sh,seq,nq")
	flags.Int32Var(&s.weight, "weight", 1, "An integer specifying the capacity of server relative to others in the pool")
	//flags.Int32Var(s.masqueradeBit, "iptables-masquerade-bit", Int32PtrDerefOr(s.masqueradeBit, 14), "If using the pure iptables proxy, the bit of the fwmark space to mark packets requiring SNAT with.  Must be within the range [0, 31].")
	flags.BoolVar(&s.masqueradeAll, "masquerade-all", s.masqueradeAll, "If using the pure iptables proxy, SNAT all traffic sent via Service cluster IPs (this not commonly needed)")
}

func interfaceAddresses() []string {
	ifacesAddress, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	var addresses []string
	for _, addr := range ifacesAddress {
		// TODO: Ignore interfaces in PodCIDR or ClusterCIDR
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			panic(err)
		}
		// I want to deal only with IPv4 right now...
		if ipv4 := ip.To4(); ipv4 == nil {
			continue
		}

		addresses = append(addresses, ip.String())
	}
	return addresses
}
