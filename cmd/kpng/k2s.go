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
	"fmt"
	"os"

	"sigs.k8s.io/kpng/cmd/kpng/storecmds"

	"github.com/spf13/cobra"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// this depends on the kpng server to run the integrated app
	"sigs.k8s.io/kpng/server/jobs/kube2store"
	"sigs.k8s.io/kpng/server/proxystore"
)

// FIXME separate package
var (
	kubeConfig string
	kubeServer string
	k2sCfg     = &kube2store.Config{}
)

func kube2storeCmd() *cobra.Command {
	// kube to * command
	k2sCmd := &cobra.Command{
		Use:   "kube",
		Short: "watch Kubernetes API to the globalv1 state",
	}

	flags := k2sCmd.PersistentFlags()
	flags.StringVar(&kubeConfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster. Defaults to envvar KUBECONFIG.")
	flags.StringVar(&kubeServer, "server", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	k2sCfg.BindFlags(k2sCmd.PersistentFlags())
	k2sCmd.AddCommand(storecmds.Commands(setupKube2store)...)

	return k2sCmd
}

func setupKube2store() (ctx context.Context, store *proxystore.Store, err error) {
	ctx = setupGlobal()

	// setup k8s client
	if kubeConfig == "" {
		kubeConfig = os.Getenv("KUBECONFIG")
	}

	cfg, err := clientcmd.BuildConfigFromFlags(kubeServer, kubeConfig)
	if err != nil {
		err = fmt.Errorf("Error building kubeconfig: %w", err)
		return
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		err = fmt.Errorf("Error building kubernetes clientset: %w", err)
		return
	}

	// create the store
	store = proxystore.New()

	// start kube2store
	go kube2store.Job{
		Kube:   kubeClient,
		Store:  store,
		Config: k2sCfg,
	}.Run(ctx)

	return
}
