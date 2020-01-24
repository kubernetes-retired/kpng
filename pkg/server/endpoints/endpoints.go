package endpoints

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
)

var _ localnetv1.EndpointsServer = &Server{}

func (s *Server) Next(filter *localnetv1.NextFilter, res localnetv1.Endpoints_NextServer) (err error) {
	if filter.InstanceID == s.InstanceID {
		s.Correlator.WaitRev(filter.Rev)
	}

	// TODO

	return status.Error(codes.Unimplemented, "unimplemented")
}
