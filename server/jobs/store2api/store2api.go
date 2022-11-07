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

package store2api

import (
	"context"
	"crypto/tls"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"sigs.k8s.io/kpng/client/tlsflags"
	"sigs.k8s.io/kpng/server/pkg/server"
	"sigs.k8s.io/kpng/server/pkg/server/endpoints"
	"sigs.k8s.io/kpng/server/pkg/server/global"
	"sigs.k8s.io/kpng/server/proxystore"
)

type Config struct {
	BindSpec  string
	GlobalAPI bool
	LocalAPI  bool
	TLS       *tlsflags.Flags
}

func (c *Config) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&c.BindSpec, "listen", "tcp://:12090", "serve globalv1 API")
	flags.BoolVar(&c.GlobalAPI, "globalv1-api", true, "serve globalv1 API")
	flags.BoolVar(&c.LocalAPI, "local-api", true, "serve local API")

	if c.TLS == nil {
		c.TLS = &tlsflags.Flags{}
	}

	c.TLS.Bind(flags, "listen-")
}

type Job struct {
	Store  *proxystore.Store
	Config *Config
}

func (j *Job) Run(ctx context.Context) error {
	lis := server.MustListen(j.Config.BindSpec)

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
		global.Setup(srv, j.Store)
	}
	if j.Config.LocalAPI {
		endpoints.Setup(srv, j.Store)
	}

	// handle exit
	go func() {
		_, _ = <-ctx.Done()
		srv.Stop()
	}()

	return srv.Serve(lis)
}
