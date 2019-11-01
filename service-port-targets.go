package main

import (
	"fmt"
	"net"

	"k8s.io/kube-localnet-api/pkg/diffstore"
)

var servicePortTargetsStore = diffstore.New()

type ServicePortDestinations struct {
	Protocol     string   `json:"proto"`
	PortName     string   `json:"name"`
	NodePort     int32    `json:"nodePort"`
	Destination  IPPort   `json:"dst"`
	External     bool     `json:"ext"`
	Destinations []IPPort `json:"dsts"`
}

func updateServicePortTargets(change serviceEndpointsChange) {
	if change.Service == nil || change.Endpoints == nil {
		servicePortTargetsStore.Delete(change.Prefix())
		return
	}

	tx := servicePortTargetsStore.Begin(change.Prefix())

	for _, ipDef := range change.ServiceIPs() {
		for _, sourcePort := range change.Service.Spec.Ports {
			spd := &ServicePortDestinations{
				Protocol:     string(sourcePort.Protocol),
				PortName:     sourcePort.Name,
				NodePort:     sourcePort.NodePort,
				Destination:  IPPort{ipDef.IP, sourcePort.Port},
				External:     ipDef.External,
				Destinations: make([]IPPort, 0, len(change.Endpoints.Subsets)),
			}
			for _, subset := range change.Endpoints.Subsets {
				for _, targetPort := range subset.Ports {
					if sourcePort.Protocol != targetPort.Protocol {
						continue
					}
					if v := sourcePort.TargetPort.IntVal; v != 0 && targetPort.Port != v {
						continue
					}
					if v := sourcePort.TargetPort.StrVal; len(v) != 0 && targetPort.Name != v {
						continue
					}

					for _, addr := range subset.Addresses {
						ip := net.ParseIP(addr.IP)
						if ip == nil {
							continue
						}

						spd.Destinations = append(spd.Destinations, IPPort{ip, targetPort.Port})
					}
				}
			}

			tx.AddJSON(fmt.Sprintf("%v:%d/%s", spd.Destination.IP, spd.Destination.Port, spd.Protocol), spd)
		}
	}

	logChanges("service port targets", tx.Apply())
}
