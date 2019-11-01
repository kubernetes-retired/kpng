package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	masterURL  string
	kubeconfig string

	quitCh = make(chan struct{}, 1)
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

func main() {
	klog.InitFlags(flag.CommandLine)

	cmd := cobra.Command{
		Use: "localnet-api",
		Run: run,
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		<-ch
		klog.Info("shutting down...")
		close(quitCh)
		os.Exit(0) // FIXME quitCh doesn't make app exits...
	}()

	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}

func run(_ *cobra.Command, _ []string) {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Errorf("Error building kubeconfig: %s", err.Error())
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Errorf("Error building kubernetes clientset: %s", err.Error())
		os.Exit(1)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

	controller := &controller{kubeClient, kubeInformerFactory}

	go localnetExtIptables()

	kubeInformerFactory.Start(quitCh)
	controller.Run(quitCh)
}
