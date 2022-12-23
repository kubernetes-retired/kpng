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
	"sigs.k8s.io/kpng/server/proxystore"

	"sigs.k8s.io/kpng/cmd/kpng/storecmds"
)

var (
	api2storeJob = &api2store.Job{
		Watch: apiwatch.Watch{TLSFlags: &tlsflags.Flags{}},
	}
)

// api2storeCmd is gives a feature to KPNG which allows you to read data from a KPNG server
// and write it to a backend.  It can be used if you dont want to watch the K8s API, but want
// to send data from another KPNG instance down to a backend.
func api2storeCmd() *cobra.Command {
	// API to * command
	api2sCmd := &cobra.Command{
		Use:   "api",
		Short: "watch kpng API to the globalv1 state",
	}

	flags := api2sCmd.PersistentFlags()
	api2storeJob.BindFlags(flags)

	context, backend, error := api2storeCmdSetup()

	run := func () {
		go api2storeJob.Run(context)
	}
	api2sCmd.AddCommand(storecmds.ToAPICmd(context, backend, error, run))
	api2sCmd.AddCommand(storecmds.ToFileCmd(context, backend, error, run ))
	api2sCmd.AddCommand(storecmds.ToLocalCmd(context, backend, error, run ))

	return api2sCmd
}

// api2storeCmdSetup generates a context , builds the in-memory storage for k8s proxy data.
// It also kicks off the job responsible for watching the KPNG internal API.
func api2storeCmdSetup() (ctx context.Context, store *proxystore.Store, err error) {
	ctx = setupGlobal()

	store = proxystore.New()

	api2storeJob.Store = store

	return
}
