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

package apiwatch

import (
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"sigs.k8s.io/kpng/client/tlsflags"
)

type Watch struct {
	Server   string
	TLSFlags *tlsflags.Flags
}

func (w *Watch) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&w.Server, "api", "127.0.0.1:12090", "Remote API server to query")
	w.TLSFlags.Bind(flags, "api-client-")
}

func (w *Watch) Dial() (conn *grpc.ClientConn, err error) {
	// connect to API
	opts := []grpc.DialOption{}

	if cfg := w.TLSFlags.Config(); cfg == nil {
		opts = append(opts, grpc.WithInsecure())
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(cfg)))
	}

	conn, err = grpc.Dial(w.Server, opts...)
	if err != nil {
		return
	}

	return
}
