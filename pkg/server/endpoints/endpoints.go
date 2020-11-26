package endpoints

import (
	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
	"m.cluseau.fr/kube-proxy2/pkg/endpoints"
	"m.cluseau.fr/kube-proxy2/pkg/proxy"
)

func Setup(srv *proxy.Server) {
	endpointsCorrelator := endpoints.NewCorrelator(srv)
	go endpointsCorrelator.Run(srv.QuitCh)

	localnetv1.RegisterEndpointsService(srv.GRPC, localnetv1.NewEndpointsService(localnetv1.UnstableEndpointsService(&Server{
		Store: srv.Store,
	})))
}
