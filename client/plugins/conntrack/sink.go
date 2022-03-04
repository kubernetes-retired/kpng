package conntrack

import (
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink"
)

type Sink struct {
	localsink.Config
	services     map[string]*localnetv1.Service
	endpoints    map[string]map[string]*localnetv1.Endpoint
	staleFlows   []Flow
	staleIPPorts []IPPort
}

func NewSink() *Sink {
	return &Sink{
		services:  make(map[string]*localnetv1.Service),
		endpoints: make(map[string]map[string]*localnetv1.Endpoint),
	}
}

func (ps *Sink) Reset() {
}

func (ps *Sink) Setup() {
}

func (ps *Sink) SetService(svc *localnetv1.Service) {
	ps.services[svc.Namespace+"/"+svc.Name] = svc
}

func (ps *Sink) DeleteService(namespace, name string) {
	delete(ps.services, namespace+"/"+name)
}

func (ps *Sink) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	service, _ := ps.services[namespace+"/"+serviceName]
	eps := len(ps.endpoints[namespace+"/"+serviceName])
	allIPs := service.IPs.All().All()
	if eps == 0 {
		if service.Type == "NodePort" {
			allIPs = append(allIPs, "node")
		}

		for _, svcIP := range allIPs {
			for _, port := range service.Ports {
				targetPort := port.Port
				if svcIP == "node" {
					targetPort = port.NodePort
				}

				if port.Port == 0 {
					continue
				}

				ps.staleIPPorts = append(ps.staleIPPorts, IPPort{Protocol: port.Protocol, DnatIP: svcIP, Port: targetPort})
			}
		}
	}
	if ps.endpoints[namespace+"/"+serviceName] == nil {
		ps.endpoints[namespace+"/"+serviceName] = make(map[string]*localnetv1.Endpoint)
	}
	ps.endpoints[namespace+"/"+serviceName][key] = endpoint
}

func (ps *Sink) DeleteEndpoint(namespace, serviceName, key string) {
	service, _ := ps.services[namespace+"/"+serviceName]
	ep, _ := ps.endpoints[namespace+"/"+serviceName][key]
	var targetPort int32
	for _, svcIP := range service.IPs.All().All() {
		for _, port := range service.Ports {
			for _, epIP := range ep.IPs.All() {
				targetPort = port.Port
				if port.Port == 0 {
					targetPort = int32(ep.PortMapping(port))
				}
				flow := Flow{
					IPPort:     IPPort{Protocol: port.Protocol, DnatIP: svcIP, Port: targetPort},
					EndpointIP: epIP,
					TargetPort: targetPort,
				}
				ps.staleFlows = append(ps.staleFlows, flow)
			}
		}
	}
	delete(ps.endpoints[namespace+"/"+serviceName], key)
}

func (s *Sink) Sync() {
	for _, ipPort := range s.staleIPPorts {
		cleanupIPPortEntries(ipPort)
	}
	for _, flow := range s.staleFlows {
		cleanupFlowEntries(flow)
	}
	s.staleIPPorts = nil
	s.staleFlows = nil
}
