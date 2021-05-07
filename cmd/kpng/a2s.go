package main

import (
	"context"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/jobs/api2store"
	"sigs.k8s.io/kpng/pkg/cmd/storecmds"
	"sigs.k8s.io/kpng/pkg/proxystore"
	"sigs.k8s.io/kpng/pkg/tlsflags"
)

var (
	api2storeJob = &api2store.Job{
		TLSFlags: &tlsflags.Flags{},
	}
)

func api2storeCmd() *cobra.Command {
	// API to * command
	cmd := &cobra.Command{
		Use:   "api",
		Short: "watch kpng API to the global state",
	}

	flags := cmd.PersistentFlags()
	flags.StringVar(&api2storeJob.Server, "api", "127.0.0.1:12090", "Remote API server to query")

	api2storeJob.TLSFlags.Bind(flags, "api-client-")

	cmd.AddCommand(storecmds.Commands(setupAPI2store)...)

	return cmd
}

func setupAPI2store() (ctx context.Context, store *proxystore.Store, err error) {
	ctx = setupGlobal()

	store = proxystore.New()

	api2storeJob.Store = store
	go api2storeJob.Run(ctx)

	return
}
