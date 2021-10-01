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

package proxy

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"runtime/trace"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"sigs.k8s.io/kpng/client/pkg/tlsflags"
	"sigs.k8s.io/kpng/server/pkg/proxystore"
)

var (
	kubeconfig string
	serverURL  string
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
	flagSet.StringVar(&serverURL, "server", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
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
	cfg, err := clientcmd.BuildConfigFromFlags(serverURL, kubeconfig)
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
