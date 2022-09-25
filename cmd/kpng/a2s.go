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

	"sigs.k8s.io/kpng/client/tlsflags"
	"sigs.k8s.io/kpng/server/jobs/api2store"
	"sigs.k8s.io/kpng/server/pkg/apiwatch"
	"sigs.k8s.io/kpng/server/pkg/proxystore"

	"sigs.k8s.io/kpng/cmd/kpng/storecmds"
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
