package global

import (
	"m.cluseau.fr/kube-proxy2/pkg/api/localnetv1"
	"m.cluseau.fr/kube-proxy2/pkg/proxy"
)

func Setup(srv *proxy.Server) {
	localnetv1.RegisterGlobalService(srv.GRPC, localnetv1.NewGlobalService(localnetv1.UnstableGlobalService(&Server{
		Store: srv.Store,
	})))
}
