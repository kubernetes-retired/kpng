package main

import (
	"context"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/server/jobs/api2store"
	"sigs.k8s.io/kpng/server/pkg/apiwatch"
	"sigs.k8s.io/kpng/server/pkg/proxystore"
	"sigs.k8s.io/kpng/client/pkg/tlsflags"
)

var (
	api2storeJob = &api2store.Job{
		Watch: apiwatch.Watch{TLSFlags: &tlsflags.Flags{}},
	}
)

func api2storeCmd() *cobra.Command {
	// API to * command
	cmd := &cobra.Command{
		Use:   "api",
		Short: "watch kpng API to the global state",
	}

	flags := cmd.PersistentFlags()
	api2storeJob.BindFlags(flags)

//	cmd.AddCommand(storecmds2.Commands(setupAPI2store)...)

	return cmd
}

func setupAPI2store() (ctx context.Context, store *proxystore.Store, err error) {
	ctx = setupGlobal()

	store = proxystore.New()

	api2storeJob.Store = store
	go api2storeJob.Run(ctx)

	return
}
