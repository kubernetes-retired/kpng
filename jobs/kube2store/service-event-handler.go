/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube2store

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog"

	"sigs.k8s.io/kpng/pkg/api/localnetv1"
	"sigs.k8s.io/kpng/pkg/proxystore"
)

type serviceEventHandler struct{ eventHandler }

func (h *serviceEventHandler) OnAdd(obj interface{}) {
	svc := obj.(*v1.Service)

	service := &localnetv1.Service{
		Namespace:   svc.Namespace,
		Name:        svc.Name,
		Type:        string(svc.Spec.Type),
		Labels:      globsFilter(svc.Labels, h.config.ServiceLabelGlobs),
		Annotations: globsFilter(svc.Annotations, h.config.ServiceAnnonationGlobs),
		// MapIP: false, // TODO could be useful for L3 managed things
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
		klog.V(3).Info("service ", service.Namespace, "/", service.Name, " topology key: ", svc.Spec.TopologyKeys)
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
		tx.DelService(svc.Namespace, svc.Name)
		h.updateSync(proxystore.Services, tx)
	})
}
