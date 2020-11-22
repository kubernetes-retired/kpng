package proxy

import (
	"flag"
	"fmt"
	"os"
	"runtime/trace"
	"time"

	"github.com/mcluseau/kube-proxy2/pkg/proxystore"
	"google.golang.org/grpc"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	kubeconfig string
	masterURL  string
	bindSpec   string
	traceFile  string

	inProcessConnBufferSize int = 32 << 10

	ManageEndpointSlices bool = true
)

func InitFlags(flagSet *flag.FlagSet) {
	klog.InitFlags(flagSet)

	flagSet.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster. Defaults to envvar KUBECONFIG.")
	flagSet.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flagSet.BoolVar(&ManageEndpointSlices, "with-endpoint-slices", ManageEndpointSlices, "Enable EndpointSlice")

	flagSet.StringVar(&traceFile, "trace", "", "trace output")
}

type Server struct {
	Client          *kubernetes.Clientset
	InformerFactory informers.SharedInformerFactory
	QuitCh          chan struct{}

	Store *proxystore.Store

	GRPC *grpc.Server

	traceFile *os.File
}

func NewServer() (srv *Server, err error) {
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	srv = &Server{
		QuitCh: make(chan struct{}, 1),
		GRPC:   grpc.NewServer(),
		Store:  proxystore.New(),
	}

	if traceFile != "" {
		f, err := os.Create(traceFile)
		if err != nil {
			klog.Fatal("failed to create requested trace file: ", err)
		}

		srv.traceFile = f

		if err = trace.Start(f); err != nil {
			klog.Fatal("failed to start trace: ", err)
		}

		klog.Info("tracing to ", traceFile)
	}

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Error building kubeconfig: %s", err.Error())
	}

	srv.Client, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building kubernetes clientset: %s", err.Error())
	}

	srv.InformerFactory = informers.NewSharedInformerFactory(srv.Client, time.Second*30)
	srv.InformerFactory.Start(srv.QuitCh)

	return
}

func (s *Server) Stop() {
	klog.Info("server stopping")

	if s.traceFile != nil {
		klog.Info()
		trace.Stop()
		s.traceFile.Close()
		klog.Info("trace closed")
	}

	close(s.QuitCh)

	s.GRPC.Stop()
}
