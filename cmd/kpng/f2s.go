/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/server/jobs/file2store"
	"sigs.k8s.io/kpng/server/proxystore"

	"sigs.k8s.io/kpng/cmd/kpng/builder"
)

// FIXME separate package
var f2sInput string

// file2storeCmd is a command that will read data from file, allowing you to locally simulate
// a networking model easily and send it to a store (i.e. a backend) of your choosing.  Its commonly
// used for local development against a known kubernetes networking statespace (see the example in the doc/)
// folder of an input YAML that works with this command.
func file2storeCmd() *cobra.Command {
	// file to * command
	f2sCmd := &cobra.Command{
		Use:   "file",
		Short: "poll a file to the globalv1 state",
	}

	flags := f2sCmd.PersistentFlags()
	flags.StringVarP(&f2sInput, "input", "i", "globalv1-state.yaml", "Input file for the globalv1-state")

	ctx := setupGlobal()
	store := proxystore.New()
	run := func() {
		file2storeCmdRun(ctx, store)
	}
	f2sCmd.AddCommand(builder.ToAPICmd(ctx, store, nil, run))
	f2sCmd.AddCommand(builder.ToFileCmd(ctx, store, nil, run))
	f2sCmd.AddCommand(builder.ToLocalCmd(ctx, store, nil, run))

	return f2sCmd
}

// file2storeCmdRun kicks off the file2store job.
func file2storeCmdRun(ctx context.Context, store *proxystore.Store) {
	f2s := &file2store.Job{
		FilePath: f2sInput,
		Store:    store,
	}
	f2s.Run(ctx)
}
