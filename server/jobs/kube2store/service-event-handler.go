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
	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/server/pkg/proxystore"
)

type serviceEventHandler struct{ eventHandler }

func (h *serviceEventHandler) onChange(obj interface{}) {
	svc := obj.(*v1.Service)

	internalTrafficPolicy := v1.ServiceInternalTrafficPolicyCluster
	if svc.Spec.InternalTrafficPolicy != nil {
		internalTrafficPolicy = *svc.Spec.InternalTrafficPolicy
	}
	// build the service
	service := &localnetv1.Service{
		Namespace:   svc.Namespace,
		Name:        svc.Name,
		Type:        string(svc.Spec.Type),
		Labels:      globsFilter(svc.Labels, h.config.ServiceLabelGlobs),
		Annotations: globsFilter(svc.Annotations, h.config.ServiceAnnonationGlobs),
		IPs: &localnetv1.ServiceIPs{
			ClusterIPs:  &localnetv1.IPSet{},
			ExternalIPs: localnetv1.NewIPSet(svc.Spec.ExternalIPs...),
		},
		ExternalTrafficToLocal: svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal,
		InternalTrafficToLocal: internalTrafficPolicy == v1.ServiceInternalTrafficPolicyLocal,
	}

	// extract cluster IPs with backward compatibility (k8s before ClusterIPs)
	clusterIPs := []string{}
	if len(svc.Spec.ClusterIPs) == 0 {
		if svc.Spec.ClusterIP != "" {
			clusterIPs = []string{svc.Spec.ClusterIP}
		}
	} else {
		clusterIPs = svc.Spec.ClusterIPs
	}

	for _, ip := range clusterIPs {
		if ip == "None" {
			service.IPs.Headless = true
		} else {
			service.IPs.ClusterIPs.Add(ip)
		}
	}

	// session affinity info
	switch svc.Spec.SessionAffinity {
	case "ClientIP":
		cfg := svc.Spec.SessionAffinityConfig.ClientIP
		service.SessionAffinity = &localnetv1.Service_ClientIP{
			ClientIP: &localnetv1.ClientIPAffinity{
				TimeoutSeconds: *cfg.TimeoutSeconds,
			},
		}
	}

	// load balancer IPs
	if len(svc.Status.LoadBalancer.Ingress) != 0 {
		ips := localnetv1.NewIPSet()
		for _, ingress := range svc.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				ips.Add(ingress.IP)
			}
		}
		service.IPs.LoadBalancerIPs = ips
	}

	// load balancer source ranges
	if len(svc.Spec.LoadBalancerSourceRanges) != 0 {
		service.IPFilters = append(service.IPFilters, &localnetv1.IPFilter{
			SourceRanges: svc.Spec.LoadBalancerSourceRanges,
		})
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
			p.TargetPortName = port.TargetPort.StrVal
		}

		service.Ports = append(service.Ports, p)
	}

	h.s.Update(func(tx *proxystore.Tx) {
		klog.V(3).Info("service ", service.Namespace, "/", service.Name)
		tx.SetService(service)
		h.updateSync(proxystore.Services, tx)
	})
}

func (h *serviceEventHandler) OnAdd(obj interface{}) {
	h.onChange(obj)
}

func (h *serviceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	h.onChange(newObj)
}

func (h *serviceEventHandler) OnDelete(oldObj interface{}) {
	svc := oldObj.(*v1.Service)

	h.s.Update(func(tx *proxystore.Tx) {
		tx.DelService(svc.Namespace, svc.Name)
		h.updateSync(proxystore.Services, tx)
	})
}
