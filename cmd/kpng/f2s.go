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
	"sigs.k8s.io/kpng/server/jobs/kube2store"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kpng/server/jobs/file2store"
	"sigs.k8s.io/kpng/server/proxystore"

	"sigs.k8s.io/kpng/cmd/kpng/storecmds"
)

// FIXME separate package
var f2sInput string

// file2storeCmd is a command that will read data from file, allowing you to locally simulate
// a networking model easily and send it to a store (i.e. a backend) of your choosing.  Its commonly
// used for local development against a known kubernetes networking statespace (see the example in the doc/)
// folder of an input YAML that works with this command.
func file2storeCmd() *cobra.Command {
	// file to * command
	k2sCmd := &cobra.Command{
		Use:   "file",
		Short: "poll a file to the globalv1 state",
	}

	flags := k2sCmd.PersistentFlags()
	flags.StringVarP(&f2sInput, "input", "i", "globalv1-state.yaml", "Input file for the globalv1-state")

	// k2sCfg is the configuration of how we interact w/ and watch the K8s APIServer
	k2sCfg := &kube2store.K8sConfig{}
	k2sCfg.BindFlags(k2sCmd.PersistentFlags())

	context, backend, error := file2storeCmdSetup(k2sCfg)

	run := func(){
		go (&file2store.Job{
			FilePath: f2sInput,
			Store:    backend,
		}).Run(context)

	}
	k2sCmd.AddCommand(storecmds.ToAPICmd(context, backend, error, run ))
	k2sCmd.AddCommand(storecmds.ToFileCmd(context, backend, error, run ))
	k2sCmd.AddCommand(storecmds.ToLocalCmd(context, backend, error, run ))

	return k2sCmd
}

// file2storeCmdSetup generates a context , builds the in-memory storage for k8s proxy data.
// It also kicks off the job responsible for watching the YAML File w/ K8s networking info.
func file2storeCmdSetup(k2sCfg *kube2store.K8sConfig) (ctx context.Context, store *proxystore.Store, err error) {
	ctx = setupGlobal()

	// create the store
	store = proxystore.New()

	return ctx, store, nil
}
