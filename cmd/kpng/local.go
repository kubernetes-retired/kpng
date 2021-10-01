package main

import (
	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/client/localsink"
	"sigs.k8s.io/kpng/cmd/kpng/storecmds"
	"sigs.k8s.io/kpng/server/jobs/api2local"
)

func local2sinkCmd() *cobra.Command {
	// local to * command
	cmd := &cobra.Command{
		Use:   "local",
		Short: "watch kpng API's local state",
	}

	job := api2local.New(nil)

	flags := cmd.PersistentFlags()
	job.BindFlags(flags)

	cmd.AddCommand(storecmds.LocalCmds(func(sink localsink.Sink) (err error) {
		ctx := setupGlobal()
		job.Sink = sink
		job.Run(ctx)
		return
	})...)

	return cmd
}
