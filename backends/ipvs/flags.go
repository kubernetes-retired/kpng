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

package ipvs

import (
	"net"

	"github.com/spf13/pflag"
)

var (
	flag = &pflag.FlagSet{}

	OnlyOutput  = flag.Bool("only-output", false, "Only output the ipvsadm-restore file instead of calling ipvsadm-restore")
	IPVSAdmPath = flag.String("ipvsadm", "/usr/sbin/ipvsadm", "Defines the path of ipvsadm-restore command")
	NodeAddress = flag.StringSlice("nodeport-address", interfaceAddresses(), "A comma-separated list of IPs to associate when using NodePort type. Defaults to all the Node addresses")
	// TODO: Not used yet
	IPVSExcludeCIDRS = flag.StringSlice("ipvs-exclude-cidrs", nil, "A comma-separated list of CIDR's which the ipvs proxier should not touch when cleaning up IPVS rules.")
	// TODO: Not used yet
	IPVSStrictArp = flag.Bool("ipvs-strict-arp", false, "Enable strict ARP by setting arp_ignore to 1 and arp_announce to 2")
)

func BindFlags(flags *pflag.FlagSet) {
	flags.AddFlagSet(flag)
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
