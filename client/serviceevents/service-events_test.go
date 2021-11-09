package serviceevents

import (
	"fmt"
	"strings"

	"sigs.k8s.io/kpng/api/localnetv1"
)

// fix the protobuf "I'll randomly choose to double spaces or not" bug (it's a bug right???)
func cleanStr(v fmt.Stringer) string {
	if v == nil {
		return "<nil>"
	}

	return strings.ReplaceAll(v.String(), "  ", " ")
}

type portsLsnr struct{}

func (_ portsLsnr) AddPort(svc *localnetv1.Service, port *localnetv1.PortMapping) {
	fmt.Print("ADD svc: ", cleanStr(svc), "\n    port: ", cleanStr(port), "\n")
}
func (_ portsLsnr) DeletePort(svc *localnetv1.Service, port *localnetv1.PortMapping) {
	fmt.Print("DEL svc: ", cleanStr(svc), "\n    port: ", cleanStr(port), "\n")
}

type ipsLsnr struct{}

func (_ ipsLsnr) AddIP(svc *localnetv1.Service, ip string, ipKind IPKind) {
	fmt.Print("ADD svc: ", cleanStr(svc), "\n    ip: ", ip, " (", ipKind, ")\n")
}
func (_ ipsLsnr) DeleteIP(svc *localnetv1.Service, ip string, ipKind IPKind) {
	fmt.Print("DEL svc: ", cleanStr(svc), "\n    ip: ", ip, " (", ipKind, ")\n")
}

type ipPortsLsnr struct{}

func (_ ipPortsLsnr) AddIPPort(svc *localnetv1.Service, ip string, ipKind IPKind, port *localnetv1.PortMapping) {
	fmt.Print("ADD svc: ", cleanStr(svc), "\n    ip: ", ip, " (", ipKind, ")\n    port: ", cleanStr(port), "\n")
}
func (_ ipPortsLsnr) DeleteIPPort(svc *localnetv1.Service, ip string, ipKind IPKind, port *localnetv1.PortMapping) {
	fmt.Print("DEL svc: ", cleanStr(svc), "\n    ip: ", ip, " (", ipKind, ")\n    port: ", cleanStr(port), "\n")
}

func Example() {
	sl := New()
	sl.PortsListener = portsLsnr{}
	sl.IPsListener = ipsLsnr{}
	sl.IPPortsListener = ipPortsLsnr{}

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

	fmt.Println("\nadd cluster IP")
	sl.SetService(&localnetv1.Service{
		Namespace: "ns",
		Name:      "svc-1",
		IPs: &localnetv1.ServiceIPs{
			ClusterIPs: localnetv1.NewIPSet("10.1.1.1"),
		},
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
	// DEL svc: Namespace:"ns" Name:"svc-1" Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:81}
	//     port: Protocol:TCP Port:81
	// ADD svc: Namespace:"ns" Name:"svc-1" Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     port: Protocol:TCP Port:82
	//
	// add cluster IP
	// ADD svc: Namespace:"ns" Name:"svc-1" IPs:{ClusterIPs:{V4:"10.1.1.1"}} Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     ip: 10.1.1.1 (ClusterIP)
	// ADD svc: Namespace:"ns" Name:"svc-1" IPs:{ClusterIPs:{V4:"10.1.1.1"}} Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     ip: 10.1.1.1 (ClusterIP)
	//     port: Protocol:TCP Port:80
	// ADD svc: Namespace:"ns" Name:"svc-1" IPs:{ClusterIPs:{V4:"10.1.1.1"}} Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     ip: 10.1.1.1 (ClusterIP)
	//     port: Protocol:TCP Port:82
	//
	// delete svc
	// DEL svc: Namespace:"ns" Name:"svc-1" IPs:{ClusterIPs:{V4:"10.1.1.1"}} Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     port: Protocol:TCP Port:80
	// DEL svc: Namespace:"ns" Name:"svc-1" IPs:{ClusterIPs:{V4:"10.1.1.1"}} Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     port: Protocol:TCP Port:82
	// DEL svc: Namespace:"ns" Name:"svc-1" IPs:{ClusterIPs:{V4:"10.1.1.1"}} Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     ip: 10.1.1.1 (ClusterIP)
	//     port: Protocol:TCP Port:80
	// DEL svc: Namespace:"ns" Name:"svc-1" IPs:{ClusterIPs:{V4:"10.1.1.1"}} Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     ip: 10.1.1.1 (ClusterIP)
	//     port: Protocol:TCP Port:82
	// DEL svc: Namespace:"ns" Name:"svc-1" IPs:{ClusterIPs:{V4:"10.1.1.1"}} Ports:{Protocol:TCP Port:80} Ports:{Protocol:TCP Port:82}
	//     ip: 10.1.1.1 (ClusterIP)

}
