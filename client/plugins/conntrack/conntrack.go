package conntrack

import (
	"sync"

	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/diffstore2"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

type Leaf = diffstore2.AnyLeaf[Flow]
type IPPortLeaf = diffstore2.AnyLeaf[IPPort]

type Conntrack struct {
	once  sync.Once
	flows *diffstore2.Store[string, *Leaf]

	// ipPorts has all the [svc IP, port] *with* endpoints
	ipPorts *diffstore2.Store[string, *IPPortLeaf]
}

var _ fullstate.Callback = (&Conntrack{}).Callback

func New() Conntrack {
	return Conntrack{
		flows:   diffstore2.NewAnyStore[string, Flow](func(a, b Flow) bool { return false }),
		ipPorts: diffstore2.NewAnyStore[string, IPPort](func(a, b IPPort) bool { return false }),
	}
}

func (ct Conntrack) reset() {
	ct.flows.Reset()
	ct.ipPorts.Reset()
}
func (ct Conntrack) done() {
	ct.flows.Done()
	ct.ipPorts.Done()
}

func (ct Conntrack) Callback(ch <-chan *client.ServiceEndpoints) {
	defer ct.reset()

	ct.once.Do(setupConntrack)

	for seps := range ch {
		allIPs := seps.Service.IPs.All().All()

		if seps.Service.Type == "NodePort" {
			allIPs = append(allIPs, "node")
		}

		for _, svcIP := range allIPs {
			for _, svcPort := range seps.Service.Ports {
				port := svcPort.Port

				if svcIP == "node" {
					port = svcPort.NodePort
				}

				if port == 0 {
					continue
				}

				ipp := IPPort{
					Protocol: svcPort.Protocol,
					DnatIP:   svcIP,
					Port:     port,
				}

				hasEndpoints := false

				for _, ep := range seps.Endpoints {
					for _, epIP := range ep.IPs.All() {
						flow := Flow{
							IPPort:     ipp,
							EndpointIP: epIP,
							TargetPort: ep.PortMapping(svcPort),
						}

						if flow.TargetPort == 0 {
							continue // no target port found
						}

						ct.flows.Get(flow.Key()).Set(flow)

						hasEndpoints = true
					}
				}

				if hasEndpoints {
					ct.ipPorts.Get(ipp.Key()).Set(ipp)
				}
			}
		}
	}

	ct.done()

	for _, item := range ct.ipPorts.Changed() {
		if item.Created() {
			cleanupIPPortEntries(item.Value().Get())
		}
	}

	for _, item := range ct.flows.Deleted() {
		cleanupFlowEntries(item.Value().Get())
	}
}
