package main

import (
	"context"
	api2store2 "sigs.k8s.io/kpng/server/jobs/api2store"
	apiwatch2 "sigs.k8s.io/kpng/server/pkg/apiwatch"
//	storecmds2 "sigs.k8s.io/kpng/server/pkg/cmd/storecmds"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
	tlsflags2 "sigs.k8s.io/kpng/server/pkg/tlsflags"

	"github.com/spf13/cobra"
)

var (
	api2storeJob = &api2store2.Job{
		Watch: apiwatch2.Watch{TLSFlags: &tlsflags2.Flags{}},
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

func setupAPI2store() (ctx context.Context, store *proxystore2.Store, err error) {
	ctx = setupGlobal()

	store = proxystore2.New()

	api2storeJob.Store = store
	go api2storeJob.Run(ctx)

	return
}
