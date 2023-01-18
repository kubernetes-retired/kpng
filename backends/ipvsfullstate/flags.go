package ipvsfullsate

import (
	"net"

	"github.com/spf13/pflag"
)

var (
	// IPVS ipvs sink flags
	BackendFlags = &pflag.FlagSet{}

	DryRun                = BackendFlags.Bool("dry-run", false, "dry run (print instead of applying)")
	NodeAddresses         = BackendFlags.StringArray("node-address", interfaceAddresses(), "A comma-separated list of IPs to associate when using NodePort type. Defaults to all the Node addresses")
	IPVSSchedulingMethod  = BackendFlags.String("scheduling-method", "rr", "Algorithm for allocating TCP conn & UDP datagrams to real servers. Values: rr,wrr,lc,wlc,lblc,lblcr,dh,sh,seq,nq")
	IPVSDestinationWeight = BackendFlags.Int32("weight", 1, "An integer specifying the capacity of server relative to others in the pool")
	// MasqueradeAll
	// flags.Int32Var(s.masqueradeBit, "iptables-masquerade-bit", Int32PtrDerefOr(s.masqueradeBit, 14), "If using the pure iptables proxy, the bit of the fwmark space to mark packets requiring SNAT with.  Must be within the range [0, 31].")
	MasqueradeAll = BackendFlags.Bool("masquerade-all", false, "If using the pure iptables proxy, SNAT all traffic sent via Service cluster IPs (this not commonly needed)")
)

func BindFlags(flags *pflag.FlagSet) {
	flags.AddFlagSet(BackendFlags)
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

		// only IPv4 for now
		if ipv4 := ip.To4(); ipv4 == nil {
			continue
		}

		addresses = append(addresses, ip.String())
	}
	return addresses
}
