package serviceevents

import (
	"sigs.k8s.io/kpng/api/localnetv1"
)

type PortsListener interface {
	AddPort(svc *localnetv1.Service, port *localnetv1.PortMapping)
	DeletePort(svc *localnetv1.Service, port *localnetv1.PortMapping)
}

type IPsListener interface {
	AddIP(svc *localnetv1.Service, ip string, ipKind IPKind)
	DeleteIP(svc *localnetv1.Service, ip string, ipKind IPKind)
}

type IPPortsListener interface {
	AddIPPort(svc *localnetv1.Service, ip string, ipKind IPKind, port *localnetv1.PortMapping)
	DeleteIPPort(svc *localnetv1.Service, ip string, ipKind IPKind, port *localnetv1.PortMapping)
}

type TrafficPolicyListener interface {
	EnableTrafficPolicy(svc *localnetv1.Service, policyKind TrafficPolicyKind)
	DisableTrafficPolicy(svc *localnetv1.Service, policyKind TrafficPolicyKind)
}

type SessionAffinityListener interface {
	EnableSessionAffinity(svc *localnetv1.Service, sessionAffinity SessionAffinity)
	DisableSessionAffinity(svc *localnetv1.Service)
}

// ServicesListener analyzes updates to the Service set and produced detailed
// events about the changes.
//
// The analyze producing events is only called if the corresponding listener
// is present, so no cost is associated with ignored events.
//
// Event call order is:
// - AddPort
// - DeletePort
// - AddIP
// - AddIPPort
// - DeleteIPPort
// - DeleteIP
//
type ServicesListener struct {
	PortsListener           PortsListener
	IPsListener             IPsListener
	IPPortsListener         IPPortsListener
	TrafficPolicyListener   TrafficPolicyListener
	SessionAffinityListener SessionAffinityListener

	services map[string]*localnetv1.Service
}

// New creates a new ServicesListener.
//
// Reminder: you need to associate listeners for this listener to be useful.
//
func New() *ServicesListener {
	return &ServicesListener{
		services: map[string]*localnetv1.Service{},
	}
}

// SetService is called when a service is added or updated
func (sl *ServicesListener) SetService(svc *localnetv1.Service) {
	svcKey := svc.Namespace + "/" + svc.Name

	prevSvc := sl.services[svcKey]

	sl.services[svcKey] = svc

	sl.diff(prevSvc, svc)
}

// DeleteService is called when a service is deleted
func (sl *ServicesListener) DeleteService(namespace, name string) {
	svcKey := namespace + "/" + name
	svc, ok := sl.services[svcKey]
	if !ok {
		return // already removed
	}

	delete(sl.services, svcKey)

	sl.diff(svc, nil)
}

func (sl *ServicesListener) diff(prevSvc, currSvc *localnetv1.Service) {
	var prevPorts, currPorts []*localnetv1.PortMapping

	if prevSvc != nil {
		prevPorts = prevSvc.Ports
	}
	if currSvc != nil {
		currPorts = currSvc.Ports
	}

	if sl.PortsListener != nil {
		Diff{
			SameKey: func(pi, ci int) bool {
				return samePort(prevPorts[pi], currPorts[ci])
			},
			Added:   func(ci int) { sl.PortsListener.AddPort(currSvc, currPorts[ci]) },
			Updated: func(_, _ int) {},
			Deleted: func(pi int) { sl.PortsListener.DeletePort(prevSvc, prevPorts[pi]) },
		}.SlicesLen(len(prevPorts), len(currPorts))
	}

	ipsExtractors := []struct {
		ipKind IPKind
		getIPs func(svc *localnetv1.Service) *localnetv1.IPSet
	}{
		{ClusterIP, func(svc *localnetv1.Service) *localnetv1.IPSet {
			if svc.IPs == nil {
				return nil
			}
			return svc.IPs.ClusterIPs
		}},
		{ExternalIP, func(svc *localnetv1.Service) *localnetv1.IPSet {
			if svc.IPs == nil {
				return nil
			}
			return svc.IPs.ExternalIPs
		}},
		{LoadBalancerIP, func(svc *localnetv1.Service) *localnetv1.IPSet {
			if svc.IPs == nil {
				return nil
			}
			return svc.IPs.LoadBalancerIPs
		}},
	}

	deferredCalls := make([]func(), 0)

	if sl.IPsListener != nil {
		for _, ext := range ipsExtractors {
			ext := ext

			var prevIPs, currIPs []string

			if prevSvc != nil {
				prevIPs = ext.getIPs(prevSvc).All()
			}
			if currSvc != nil {
				currIPs = ext.getIPs(currSvc).All()
			}

			Diff{
				SameKey: func(pi, ci int) bool {
					return prevIPs[pi] == currIPs[ci]
				},
				Added: func(ci int) {
					sl.IPsListener.AddIP(currSvc, currIPs[ci], ext.ipKind)
				},
				Updated: func(_, _ int) {},
				Deleted: func(pi int) {
					deferredCalls = append(deferredCalls, func() { sl.IPsListener.DeleteIP(prevSvc, prevIPs[pi], ext.ipKind) })
				},
			}.SlicesLen(len(prevIPs), len(currIPs))
		}
	}

	if sl.IPPortsListener != nil {
		for _, ext := range ipsExtractors {
			ext := ext

			type ipPort struct {
				ip   string
				port *localnetv1.PortMapping
			}

			combine := func(svc *localnetv1.Service) []ipPort {
				if svc == nil {
					return nil
				}

				ips := ext.getIPs(svc).All()

				pairs := make([]ipPort, 0, len(ips)*len(svc.Ports))
				for _, ip := range ips {
					for _, port := range svc.Ports {
						pairs = append(pairs, ipPort{ip, port})
					}
				}
				return pairs
			}

			prevs := combine(prevSvc)
			currs := combine(currSvc)

			Diff{
				SameKey: func(pi, ci int) bool {
					return prevs[pi].ip == currs[ci].ip && samePort(prevs[pi].port, currs[ci].port)
				},
				Added:   func(ci int) { sl.IPPortsListener.AddIPPort(currSvc, currs[ci].ip, ext.ipKind, currs[ci].port) },
				Updated: func(_, _ int) {},
				Deleted: func(pi int) { sl.IPPortsListener.DeleteIPPort(prevSvc, prevs[pi].ip, ext.ipKind, prevs[pi].port) },
			}.SlicesLen(len(prevs), len(currs))
		}
	}

	if sl.TrafficPolicyListener != nil {
		// ServiceInternalTrafficPolicy is applicable only to ClusterIP
		// TODO feature gate code needs to be added. Currently it throws error
		// due to old k8s version
		sl.checkTrafficPolicy(prevSvc, currSvc, TrafficPolicyInternal)

		// ServiceExternalTrafficPolicy is applicable to all service IPs
		// except ClusterIP
		sl.checkTrafficPolicy(prevSvc, currSvc, TrafficPolicyExternal)
	}

	if sl.SessionAffinityListener != nil {
		var prevSessAff, currSessAff SessionAffinity
		if prevSvc != nil {
			prevSessAff = GetSessionAffinity(prevSvc.SessionAffinity)
		}
		if currSvc != nil {
			currSessAff = GetSessionAffinity(currSvc.SessionAffinity)
		}
		if prevSessAff.ClientIP == nil && currSessAff.ClientIP != nil {
			sl.SessionAffinityListener.EnableSessionAffinity(currSvc, currSessAff)
		}

		if prevSessAff.ClientIP != nil && currSessAff.ClientIP == nil {
			sl.SessionAffinityListener.DisableSessionAffinity(prevSvc)
		}
	}

	for _, deferredCall := range deferredCalls {
		deferredCall()
	}
}

func samePort(p1, p2 *localnetv1.PortMapping) bool {
	return p1.Protocol == p2.Protocol && p1.Port == p2.Port
}

// SessionAffinity contains data about assinged session affinity
type SessionAffinity struct {
	ClientIP *localnetv1.Service_ClientIP
}

func GetSessionAffinity(affinity interface{}) SessionAffinity {
	var sessionAffinity SessionAffinity
	switch affinity.(type) {
	case *localnetv1.Service_ClientIP:
		sessionAffinity.ClientIP = affinity.(*localnetv1.Service_ClientIP)
	}
	return sessionAffinity
}

func (sl *ServicesListener) checkTrafficPolicy(prev, curr *localnetv1.Service, polKind TrafficPolicyKind) {
	var previousPolicy, currentPolicy bool = false, false
	switch polKind {
	case TrafficPolicyInternal:
		if prev != nil {
			previousPolicy = prev.InternalTrafficToLocal
		}
		if curr != nil {
			currentPolicy = curr.InternalTrafficToLocal
		}
		break
	case TrafficPolicyExternal:
		if prev != nil {
			previousPolicy = prev.ExternalTrafficToLocal
		}
		if curr != nil {
			currentPolicy = curr.ExternalTrafficToLocal
		}
		break
	}

	if previousPolicy == false && currentPolicy == true {
		sl.TrafficPolicyListener.EnableTrafficPolicy(curr, polKind)
	}

	if previousPolicy == true && currentPolicy == false {
		sl.TrafficPolicyListener.DisableTrafficPolicy(curr, polKind)
	}
}
