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

	// kubeConfig is the kubeConfig file for the apiserver
	kubeConfig string

	// kubeServer is the location of the external k8s apiserver.  If this is empty, we resort
	// to in-cluster configuration using internal pod service accounts.
	kubeServer string
)

// kube2storeCmd generates the kube-to-store command, which is the "normal" way to run KPNG,
// wherein you read data in from kubernetes, and push it into a store (file, API, or local backend such as NFT).
func kube2storeCmd() *cobra.Command {
	// kube to * command
	k2sCmd := &cobra.Command{
		Use:   "kube",
		Short: "watch Kubernetes API to the globalv1 state",
	}

	flags := k2sCmd.PersistentFlags()
	flags.StringVar(&kubeConfig, "kubeconfig", "", "GetPath to a kubeconfig. Only required if out-of-cluster. Defaults to envvar KUBECONFIG.")
	flags.StringVar(&kubeServer, "server", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	// k2sCfg is the configuration of how we interact w/ and watch the K8s APIServer
	k2sCfg := &kube2store.K8sConfig{}
	k2sCfg.BindFlags(k2sCmd.PersistentFlags())

	context, backend, error, kubeClient := kube2storeCmdSetup(k2sCfg)

	run := func() {
		kube2storeCmdRun(kubeClient, backend, k2sCfg, context)
	}
	k2sCmd.AddCommand(storecmds.ToAPICmd(context, backend, error, run))
	k2sCmd.AddCommand(storecmds.ToFileCmd(context, backend, error, run))
	k2sCmd.AddCommand(storecmds.ToLocalCmd(context, backend, error, run))

	return k2sCmd
}

// kube2storeCmdSetup generates a context , builds the in-memory storage for k8s proxy data.
// It also kicks off the job responsible for watching the K8s APIServer.
func kube2storeCmdSetup(k2sCfg *kube2store.K8sConfig) (ctx context.Context, store *proxystore.Store, err error, kubeClient *kubernetes.Clientset) {
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

	kCli, err := kubernetes.NewForConfig(cfg)
	kubeClient = kCli
	if err != nil {
		err = fmt.Errorf("Error building kubernetes clientset: %w", err)
		return
	}

	// create the store
	store = proxystore.New()

	return
}

func kube2storeCmdRun(kubeClient *kubernetes.Clientset, backend *proxystore.Store, k2sCfg *kube2store.K8sConfig, ctx context.Context) {
	// start kube2store
	go kube2store.Job{
		Kube:   kubeClient,
		Store:  backend,
		Config: k2sCfg,
	}.Run(ctx)
}
