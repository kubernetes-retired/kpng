package main

import (
	"context"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/server/jobs/file2store"
	"sigs.k8s.io/kpng/server/pkg/proxystore"
)

// FIXME separate package
var f2sInput string

func file2storeCmd() *cobra.Command {
	// file to * command
	k2sCmd := &cobra.Command{
		Use:   "file",
		Short: "poll a file to the global state",
	}

	flags := k2sCmd.PersistentFlags()
	flags.StringVarP(&f2sInput, "input", "i", "global-state.yaml", "Input file for the global-state")

	k2sCfg.BindFlags(k2sCmd.PersistentFlags())
	//	k2sCmd.AddCommand(storecmds2.Commands(setupFile2store)...)

	return k2sCmd
}

func setupFile2store() (ctx context.Context, store *proxystore.Store, err error) {
	ctx = setupGlobal()

	// create the store
	store = proxystore.New()

	go (&file2store.Job{
		FilePath: f2sInput,
		Store:    store,
	}).Run(ctx)

	return ctx, store, nil
}
