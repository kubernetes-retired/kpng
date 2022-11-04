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

	"sigs.k8s.io/kpng/cmd/kpng/storecmds"
)

// FIXME separate package
var f2sInput string

func file2storeCmd() *cobra.Command {
	// file to * command
	k2sCmd := &cobra.Command{
		Use:   "file",
		Short: "poll a file to the globalv1 state",
	}

	flags := k2sCmd.PersistentFlags()
	flags.StringVarP(&f2sInput, "input", "i", "globalv1-state.yaml", "Input file for the globalv1-state")

	k2sCfg.BindFlags(k2sCmd.PersistentFlags())
	k2sCmd.AddCommand(storecmds.Commands(setupFile2store)...)

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
