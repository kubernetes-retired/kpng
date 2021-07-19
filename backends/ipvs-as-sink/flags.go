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
