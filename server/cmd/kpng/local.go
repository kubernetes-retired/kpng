package main

import (
	"github.com/spf13/cobra"
	api2local2 "sigs.k8s.io/kpng/server/jobs/api2local"
	// storecmds2 "sigs.k8s.io/kpng/server/pkg/cmd/storecmds"

	"sigs.k8s.io/kpng/server/localsink"
)

func local2sinkCmd() *cobra.Command {
	// local to * command
	cmd := &cobra.Command{
		Use:   "local",
		Short: "watch kpng API's local state",
	}

	job := api2local2.New(nil)

	flags := cmd.PersistentFlags()
	job.BindFlags(flags)

	cmd.AddCommand(storecmds2.LocalCmds(func(sink localsink.Sink) (err error) {
		ctx := setupGlobal()
		job.Sink = sink
		job.Run(ctx)
		return
	})...)

	return cmd
}
