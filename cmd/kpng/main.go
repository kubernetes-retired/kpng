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
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"sigs.k8s.io/kpng/jobs/kube2store"
	"sigs.k8s.io/kpng/pkg/cmd/storecmds"
	"sigs.k8s.io/kpng/pkg/proxy"
	"sigs.k8s.io/kpng/pkg/proxystore"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

	kubeConfig string
	kubeMaster string

	k2sCfg = &kube2store.Config{}
)

func main() {
	klog.InitFlags(flag.CommandLine)

	cmd := cobra.Command{
		Use: "proxy",
	}

	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	// kube to * command
	k2sCmd := &cobra.Command{
		Use: "kube",
	}

	flags := k2sCmd.PersistentFlags()
	flags.StringVar(&kubeConfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster. Defaults to envvar KUBECONFIG.")
	flags.StringVar(&kubeMaster, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	k2sCfg.BindFlags(k2sCmd.PersistentFlags())
	k2sCmd.AddCommand(storecmds.Commands(setupKube2store)...)

	cmd.AddCommand(k2sCmd)

	// api to * command
	// TODO
	// apiCmd := &cobra.Command{
	//     Use: "api",
	// }
	// apiCmd.AddCommand(storecmds.Commands(setupAPI2store)...)

	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}

func setupGlobal() (ctx context.Context) {
	ctx, cancel := context.WithCancel(context.Background())

	// handle exit signals
	go func() {
		proxy.WaitForTermSignal()
		cancel()

		proxy.WaitForTermSignal()
		klog.Fatal("forced exit after second term signal")
		os.Exit(1)
	}()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			klog.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	return
}

func setupKube2store() (ctx context.Context, store *proxystore.Store, err error) {
	ctx = setupGlobal()

	// setup k8s client
	if kubeConfig == "" {
		kubeConfig = os.Getenv("KUBECONFIG")
	}

	cfg, err := clientcmd.BuildConfigFromFlags(kubeMaster, kubeConfig)
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
