package conntrack

import (
	"sync"

	"k8s.io/klog/v2"
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

				klog.V(1).Info("svc IP ", svcIP)

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

	klog.V(2).Info("received the new conntrack state")

	for _, item := range ct.ipPorts.Changed() {
		if item.Created() {
			ipp := item.Value().Get()
			klog.V(1).Infof("cleaning conntrack entries for new service IP:port %v", ipp)
			cleanupIPPortEntries(ipp)
		}
	}

	for _, item := range ct.flows.Deleted() {
		flow := item.Value().Get()
		klog.V(1).Infof("cleaning conntrack entries for delete flow %v", flow)
		cleanupFlowEntries(flow)
	}
}
