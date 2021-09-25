package main

import (
	"context"
	file2store2 "sigs.k8s.io/kpng/server/jobs/file2store"
//	storecmds2 "sigs.k8s.io/kpng/server/pkg/cmd/storecmds"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"

	"github.com/spf13/cobra"
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

func setupFile2store() (ctx context.Context, store *proxystore2.Store, err error) {
	ctx = setupGlobal()

	// create the store
	store = proxystore2.New()

	go (&file2store2.Job{
		FilePath: f2sInput,
		Store:    store,
	}).Run(ctx)

	return ctx, store, nil
}
