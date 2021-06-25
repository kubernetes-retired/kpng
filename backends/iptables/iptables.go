package iptables

import (
	"fmt"
	"sigs.k8s.io/kpng/client"

	"github.com/spf13/pflag"
)

var (
	flag = &pflag.FlagSet{}

	OnlyOutput = flag.Bool("only-output", false, "Only output the ipvsadm-restore file instead of calling ipvsadm-restore")
)

func PreRun() error {
	return nil
}

func BindFlags(flags *pflag.FlagSet) {
	flags.AddFlagSet(flag)
}

// Callback receives the fullstate every time, so we can make the proxier.go functionality
// by rebuilding all the state as needed.
func Callback(ch <-chan *client.ServiceEndpoints) {

	// TODO replace incoming ServiceEndpoints objects into the
	//	var ipvsCfg strings.Builder
	//	var err error
	// clusterIPs = make(map[string]interface{})

	for serviceEndpoints := range ch {
		fmt.Println()
		svc := serviceEndpoints.Service
		//	endpoints := serviceEndpoints.Endpoints

		fmt.Println(fmt.Sprintf("%v", svc))
	}
}
