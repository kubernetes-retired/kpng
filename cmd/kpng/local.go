package main

import (
	"github.com/spf13/cobra"
	"sigs.k8s.io/kpng/jobs/api2local"
	"sigs.k8s.io/kpng/localsink"
	"sigs.k8s.io/kpng/localsink/fullstate"
	"sigs.k8s.io/kpng/pkg/cmd/storecmds"
)

func local2sinkCmd() *cobra.Command {
	// local to * command
	cmd := &cobra.Command{
		Use:   "local",
		Short: "watch kpng API's local state",
	}

	cfg := &localsink.Config{}
	sink := fullstate.New(cfg)

	job := api2local.New(sink)

	flags := cmd.PersistentFlags()
	job.BindFlags(flags)
	cfg.BindFlags(flags)

	cmd.AddCommand(storecmds.BackendCmds(sink, func() (err error) {
		ctx := setupGlobal()
		job.Run(ctx)
		return
	})...)

	return cmd
}
