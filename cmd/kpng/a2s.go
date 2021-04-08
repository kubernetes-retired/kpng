package main

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/pkg/cmd/storecmds"
	"sigs.k8s.io/kpng/pkg/proxystore"
)

var (
	apiServerURL string
)

func api2storeCmd() *cobra.Command {
	// API to * command
	cmd := &cobra.Command{
		Use:   "api",
		Short: "watch Kubernetes API to the global state",
	}

	flags := cmd.PersistentFlags()
	flags.StringVar(&apiServerURL, "target", "127.0.0.1:12090", "Remote API server to query")

	cmd.AddCommand(storecmds.Commands(setupAPI2store)...)

	return cmd
}

func setupAPI2store() (ctx context.Context, store *proxystore.Store, err error) {
	ctx = setupGlobal()

	return nil, nil, errors.New("TODO") // TODO
}
