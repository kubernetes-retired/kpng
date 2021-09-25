package store2api

import (
	"context"
	"crypto/tls"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
	server2 "sigs.k8s.io/kpng/server/pkg/server"
	endpoints2 "sigs.k8s.io/kpng/server/pkg/server/endpoints"
	global2 "sigs.k8s.io/kpng/server/pkg/server/global"
	tlsflags2 "sigs.k8s.io/kpng/server/pkg/tlsflags"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	BindSpec  string
	GlobalAPI bool
	LocalAPI  bool
	TLS       *tlsflags2.Flags
}

func (c *Config) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&c.BindSpec, "listen", "tcp://:12090", "serve global API")
	flags.BoolVar(&c.GlobalAPI, "global-api", true, "serve global API")
	flags.BoolVar(&c.LocalAPI, "local-api", true, "serve local API")

	if c.TLS == nil {
		c.TLS = &tlsflags2.Flags{}
	}

	c.TLS.Bind(flags, "listen-")
}

type Job struct {
	Store  *proxystore2.Store
	Config *Config
}

func (j *Job) Run(ctx context.Context) error {
	lis := server2.MustListen(j.Config.BindSpec)

	// setup gRPC server
	var srv *grpc.Server
	if tlsCfg := j.Config.TLS.Config(); tlsCfg == nil {
		srv = grpc.NewServer()
	} else {
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		tlsCfg.ClientCAs = tlsCfg.RootCAs

		creds := credentials.NewTLS(tlsCfg)
		srv = grpc.NewServer(grpc.Creds(creds))
	}

	// setup server
	if j.Config.GlobalAPI {
		global2.Setup(srv, j.Store)
	}
	if j.Config.LocalAPI {
		endpoints2.Setup(srv, j.Store)
	}

	// handle exit
	go func() {
		_, _ = <-ctx.Done()
		srv.Stop()
	}()

	return srv.Serve(lis)
}
