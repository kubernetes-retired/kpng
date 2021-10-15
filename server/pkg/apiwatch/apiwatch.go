package apiwatch

import (
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"sigs.k8s.io/kpng/client/pkg/tlsflags"
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
