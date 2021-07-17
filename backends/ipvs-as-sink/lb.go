package ipvssink

import (
	"encoding/json"
	"net"
	"syscall"

	"github.com/google/seesaw/ipvs"
	"sigs.k8s.io/kpng/pkg/api/localnetv1"
)

type ipvsLB struct {
	IP         string
	ServiceKey string
	Port       *localnetv1.PortMapping
}

func (lb ipvsLB) String() string {
	ba, _ := json.Marshal(lb)
	return string(ba)
}

func (lb ipvsLB) ToService() ipvs.Service {
	s := ipvs.Service{
		Address:   net.ParseIP(lb.IP),
		Port:      uint16(lb.Port.Port),
		Scheduler: "rr",
	}

	switch lb.Port.Protocol {
	case localnetv1.Protocol_TCP:
		s.Protocol = syscall.IPPROTO_TCP
	case localnetv1.Protocol_UDP:
		s.Protocol = syscall.IPPROTO_UDP
	case localnetv1.Protocol_SCTP:
		s.Protocol = syscall.IPPROTO_SCTP
	}

	return s
}

func ipvsDestination(targetIP string, port *localnetv1.PortMapping) ipvs.Destination {
	return ipvs.Destination{
		Address: net.ParseIP(targetIP),
		Port:    uint16(port.TargetPort),
		Weight:  1,
	}
}

type ipvsSvcDst struct {
	Svc ipvs.Service
	Dst ipvs.Destination
}
