package proxy

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"runtime/trace"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/mcluseau/kube-proxy2/pkg/proxystore"
	"github.com/mcluseau/kube-proxy2/pkg/tlsflags"
)

var (
	kubeconfig string
	masterURL  string
	bindSpec   string
	traceFile  string

	inProcessConnBufferSize int = 32 << 10

	ManageEndpointSlices bool = true

	tlsFlags *tlsflags.Flags
)

func InitFlags(flagSet *flag.FlagSet) {
	klog.InitFlags(flagSet)

	tlsFlags = tlsflags.Bind(flagSet)

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
		Store:  proxystore.New(),
	}

	// setup tracing
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

	// setup kubernetes client
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Error building kubeconfig: %s", err.Error())
	}

	srv.Client, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building kubernetes clientset: %s", err.Error())
	}

	// setup gRPC server
	if tlsCfg := tlsFlags.Config(); tlsCfg == nil {
		srv.GRPC = grpc.NewServer()
	} else {
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		tlsCfg.ClientCAs = tlsCfg.RootCAs

		creds := credentials.NewTLS(tlsCfg)
		srv.GRPC = grpc.NewServer(grpc.Creds(creds))
	}

	// start informers
	srv.InformerFactory = informers.NewSharedInformerFactory(srv.Client, time.Second*30)
	srv.InformerFactory.Start(srv.QuitCh)

	return
}

func (s *Server) Stop() {
	klog.Info("server stopping")

	s.GRPC.Stop()

	if s.traceFile != nil {
		trace.Stop()
		s.traceFile.Close()
		klog.Info("trace closed")
	}

	close(s.QuitCh)
}
