package conntrack

import (
	"sync"

	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/diffstore2"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

type Leaf = diffstore2.AnyLeaf[Flow]

type Conntrack struct {
	once  sync.Once
	flows *diffstore2.Store[string, *Leaf]
}

var _ fullstate.Callback = (&Conntrack{}).Callback

func New() Conntrack {
	return Conntrack{
		flows: diffstore2.NewAnyStore[string, Flow](func(a, b Flow) bool { return false }),
	}
}

func (ct Conntrack) Callback(ch <-chan *client.ServiceEndpoints) {
	defer ct.flows.Reset()

	ct.once.Do(setupConntrack)

	for seps := range ch {
		for _, svcIP := range seps.Service.IPs.All().All() {
			for _, svcPort := range seps.Service.Ports {
				for _, ep := range seps.Endpoints {
					for _, epIP := range ep.IPs.All() {
						flow := Flow{
							Protocol:   svcPort.Protocol,
							DnatIP:     svcIP,
							Port:       svcPort.Port,
							EndpointIP: epIP,
							TargetPort: svcPort.TargetPort,
						}

						if targetPort, ok := ep.EndpointPortMap[svcPort.Name]; ok {
							flow.TargetPort = targetPort
						}

						ct.flows.Get(flow.Key()).Set(flow)
					}
				}
			}
		}
	}

	ct.flows.Done()

	for _, item := range ct.flows.Deleted() {
		cleanupFlowEntries(item.Value().Get())
	}
}
