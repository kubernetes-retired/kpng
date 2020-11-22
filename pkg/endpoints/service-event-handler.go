package endpoints

import (
	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/proxystore"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type serviceEventHandler struct{ eventHandler }

func (h *serviceEventHandler) OnAdd(obj interface{}) {
	svc := obj.(*v1.Service)

	service := &localnetv1.Service{
		Namespace: svc.Namespace,
		Name:      svc.Name,
		Type:      string(svc.Spec.Type),
		MapIP:     false, // TODO for headless? or no ports means all? why am I adding those questions? ;-)
		IPs: &localnetv1.ServiceIPs{
			ClusterIP:   svc.Spec.ClusterIP,
			ExternalIPs: localnetv1.NewIPSet(svc.Spec.ExternalIPs),
		},
		ExternalTrafficToLocal: svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal,
	}

	// ports information
	service.Ports = make([]*localnetv1.PortMapping, 0, len(svc.Spec.Ports))

	for _, port := range svc.Spec.Ports {
		p := &localnetv1.PortMapping{
			Name:     port.Name,
			NodePort: port.NodePort,
			Port:     port.Port,
			Protocol: localnetv1.ParseProtocol(string(port.Protocol)),
		}

		if port.TargetPort.Type == intstr.Int {
			p.TargetPort = port.TargetPort.IntVal
		} else {
			// translate name to port
			p.TargetPortName = port.TargetPort.StrVal

			/* TODO its much more complicated than that and requires pod information to resolve port names and has to be done in correlation phase and it may even be an obsolete question with EndpointsSlice

			if src.Endpoints != nil {
				for _, subset := range src.Endpoints.Subsets {
					for _, ssPort := range subset.Ports {
						//log.Print("ssport: ", ssPort.Protocol, "/", ssPort.Name, "; lookup: ", port.Protocol, "/", portName)
						if ssPort.Protocol == port.Protocol && ssPort.Name == portName {
							if p.TargetPort != 0 && p.TargetPort != port.Port {
								// FIXME not supported yet
								klog.V(1).Infof("in service %s/%s: port %v is inconsistent across endpoints (resolves to at least %d and %d)",
									src.Service.Namespace, src.Service.Name, port.TargetPort.StrVal, p.TargetPort, port.Port)
								continue portLoop
							}

							p.TargetPort = port.Port
						}
					}
				}

				if p.TargetPort == 0 {
					klog.V(1).Infof("in service %s/%s: target port %q not found", src.Service.Namespace, src.Service.Name, port.TargetPort.StrVal)
					continue portLoop
				}
			}
			*/
		}

		service.Ports = append(service.Ports, p)
	}

	h.s.Update(func(tx *proxystore.Tx) {
		tx.SetService(service, svc.Spec.TopologyKeys)
		h.updateSync(proxystore.Services, tx)
	})
}

func (h *serviceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}

func (h *serviceEventHandler) OnDelete(oldObj interface{}) {
	svc := oldObj.(*v1.Service)

	h.s.Update(func(tx *proxystore.Tx) {
		tx.DelService(svc)
		h.updateSync(proxystore.Services, tx)
	})
}
