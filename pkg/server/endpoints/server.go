package endpoints

import "github.com/mcluseau/kube-proxy2/pkg/endpoints"

type Server struct {
	InstanceID uint64
	Correlator *endpoints.Correlator
}
