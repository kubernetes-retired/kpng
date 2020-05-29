package endpoints

import (
	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/endpoints"
	"github.com/mcluseau/kube-proxy2/pkg/proxy"
)

func Setup(srv *proxy.Server) {
	endpointsCorrelator := endpoints.NewCorrelator(srv)
	go endpointsCorrelator.Run(srv.QuitCh)

	localnetv1.RegisterEndpointsServer(srv.GRPC, &Server{
		InstanceID: srv.InstanceID,
		Correlator: endpointsCorrelator,
	})
}
