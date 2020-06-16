package client

import (
	"flag"

	"github.com/spf13/pflag"
)

var _ FlagSet = flag.CommandLine
var _ FlagSet = &pflag.FlagSet{}
