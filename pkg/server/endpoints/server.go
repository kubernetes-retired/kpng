package endpoints

import (
	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/endpoints"
)

type Server struct {
	InstanceID uint64
	Correlator *endpoints.Correlator
}

var _ localnetv1.EndpointsServer = &Server{}

func (s *Server) Next(filter *localnetv1.NextFilter, res localnetv1.Endpoints_NextServer) (err error) {
	if filter.InstanceID != s.InstanceID {
		// instance changed, so anything we have is new
		filter.Rev = 0
	}

	list, rev := s.Correlator.Next(filter.Rev)

	err = res.Send(&localnetv1.NextItem{
		Next: &localnetv1.NextFilter{
			InstanceID: s.InstanceID,
			Rev:        rev + 1,
		},
	})

	if err != nil {
		return
	}

	for _, seps := range list {
		err = res.Send(&localnetv1.NextItem{
			Endpoints: seps,
		})

		if err != nil {
			return
		}
	}

	return
}
