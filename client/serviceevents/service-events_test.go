package serviceevents

import (
	"fmt"

	"sigs.k8s.io/kpng/api/localnetv1"
)

type portsLsnr struct{}

func (pl portsLsnr) AddPort(svc *localnetv1.Service, port *localnetv1.PortMapping) {
	fmt.Print("ADD svc: ", svc, "\n    port: ", port, "\n")
}
func (pl portsLsnr) DeletePort(svc *localnetv1.Service, port *localnetv1.PortMapping) {
	fmt.Print("DEL svc: ", svc, "\n    port: ", port, "\n")
}

func ExampleListener() {
	sl := New()
	sl.PortsListener = portsLsnr{}

	fmt.Println("add svc with port 80")
	sl.SetService(&localnetv1.Service{
		Namespace: "ns",
		Name:      "svc-1",
		Ports: []*localnetv1.PortMapping{
			{Protocol: localnetv1.Protocol_TCP, Port: 80},
		},
	})

	fmt.Println("\nadd port 81")
	sl.SetService(&localnetv1.Service{
		Namespace: "ns",
		Name:      "svc-1",
		Ports: []*localnetv1.PortMapping{
			{Protocol: localnetv1.Protocol_TCP, Port: 80},
			{Protocol: localnetv1.Protocol_TCP, Port: 81},
		},
	})

	fmt.Println("\nadd port 82, remove port 81")
	sl.SetService(&localnetv1.Service{
		Namespace: "ns",
		Name:      "svc-1",
		Ports: []*localnetv1.PortMapping{
			{Protocol: localnetv1.Protocol_TCP, Port: 80},
			{Protocol: localnetv1.Protocol_TCP, Port: 82},
		},
	})

	fmt.Println("\ndelete svc")
	sl.DeleteService("ns", "svc-1")

	// Output:
	// add svc with port 80
	// ADD svc: Namespace:"ns" Name:"svc-1" Ports:{Protocol:TCP Port:80}
	//     port: Protocol:TCP Port:80
	//
	// add port 81
	// ADD svc: Namespace:"ns" Name:"svc-1" Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:81}
	//     port: Protocol:TCP Port:81
	//
	// add port 82, remove port 81
	// ADD svc: Namespace:"ns" Name:"svc-1" Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     port: Protocol:TCP Port:82
	// DEL svc: Namespace:"ns" Name:"svc-1" Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     port: Protocol:TCP Port:81
	//
	// delete svc
	// DEL svc: Namespace:"ns" Name:"svc-1" Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     port: Protocol:TCP Port:80
	// DEL svc: Namespace:"ns" Name:"svc-1" Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     port: Protocol:TCP Port:82

}
