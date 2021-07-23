package ipvssink

import (
	"encoding/json"
	"net"
	"syscall"

	"github.com/google/seesaw/ipvs"
	"sigs.k8s.io/kpng/pkg/api/localnetv1"
)

type ipvsLB struct {
	IP               string
	ServiceKey       string
	ServiceType      string
	Port             *localnetv1.PortMapping
	SchedulingMethod string
}

func (lb ipvsLB) String() string {
	ba, _ := json.Marshal(lb)
	return string(ba)
}

func (lb ipvsLB) ToService() ipvs.Service {
	var port uint16
	if lb.ServiceType == ClusterIPService {
		port = uint16(lb.Port.Port)
	}
	if lb.ServiceType == NodePortService {
		port = uint16(lb.Port.NodePort)
	}
	s := ipvs.Service{
		Address:   net.ParseIP(lb.IP),
		Port:      port,
		Scheduler: lb.SchedulingMethod,
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

func ipvsDestination(targetIP string, port *localnetv1.PortMapping, epWeight int32) ipvs.Destination {
	return ipvs.Destination{
		Address: net.ParseIP(targetIP),
		Port:    uint16(port.TargetPort),
		Weight:  epWeight,
	}
}

type ipvsSvcDst struct {
	Svc ipvs.Service
	Dst ipvs.Destination
}
