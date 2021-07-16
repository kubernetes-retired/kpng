package ipvssink

import (
	"bytes"
	"log"

	"github.com/spf13/pflag"
	"sigs.k8s.io/kpng/localsink"
	"sigs.k8s.io/kpng/localsink/decoder"
	"sigs.k8s.io/kpng/localsink/filterreset"
	"sigs.k8s.io/kpng/pkg/api/localnetv1"
)

type Backend struct {
	localsink.Config

	dryRun bool

	buf *bytes.Buffer
}

var _ decoder.Interface = &Backend{}

func New() *Backend {
	return &Backend{
		buf: &bytes.Buffer{},
	}
}

func (s *Backend) Sink() localsink.Sink {
	return filterreset.New(decoder.New(s))
}

func (s *Backend) BindFlags(flags *pflag.FlagSet) {
	s.Config.BindFlags(flags)

	// real ipvs sink flags
	flags.BoolVar(&s.dryRun, "dry-run", false, "dry run (print instead of applying)")
}

func (s *Backend) Reset() { /* noop, we're wrapped in filterreset */ }

func (s *Backend) Sync() {
	log.Print("Sync()")
	// TODO
}

func (s *Backend) SetService(service *localnetv1.Service) {
	log.Printf("SetService(%v)", service)
	// TODO
}

func (s *Backend) DeleteService(namespace, name string) {
	log.Printf("DeleteService(%q, %q)", namespace, name)
	// TODO
}

func (s *Backend) SetEndpoint(namespace, serviceName, key string, endpoint *localnetv1.Endpoint) {
	log.Printf("SetEndpoint(%q, %q, %q, %v)", namespace, serviceName, key, endpoint)
	// TODO
}

func (s *Backend) DeleteEndpoint(namespace, serviceName, key string) {
	log.Printf("DeleteEndpoint(%q, %q, %q)", namespace, serviceName, key)
	// TODO
}
