package api2localdiff

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/kpng/localsink"
)

type Config struct {
	NodeName string
}

func (c *Config) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&c.NodeName, "node-name", "", "Node name override")
}

type Sink = localsink.Sink

type Job struct {
	Sink Sink
}
