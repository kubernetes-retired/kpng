package proxy

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	kubeconfig string
	masterURL  string

	inProcessConnBufferSize int = 32 << 10

	ManageEndpointSlices bool
)

func InitFlags(flagSet *flag.FlagSet) {
	klog.InitFlags(flagSet)

	flagSet.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster. Defaults to envvar KUBECONFIG.")
	flagSet.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flagSet.IntVar(&inProcessConnBufferSize, "in-process-buffer", inProcessConnBufferSize, "In-process connection buffer")
	flagSet.BoolVar(&ManageEndpointSlices, "with-endpoint-slices", ManageEndpointSlices, "Enable EndpointSlice")
}

type Server struct {
	InstanceID      uint64
	Client          *kubernetes.Clientset
	InformerFactory informers.SharedInformerFactory
	QuitCh          chan struct{}

	GRPC *grpc.Server
}

func NewServer() (srv *Server, err error) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building kubernetes clientset: %s", err.Error())
	}

	srv = &Server{
		InstanceID:      rng.Uint64(),
		Client:          kubeClient,
		InformerFactory: informers.NewSharedInformerFactory(kubeClient, time.Second*30),
		QuitCh:          make(chan struct{}, 1),
		GRPC:            grpc.NewServer(),
	}

	srv.InformerFactory.Start(srv.QuitCh)

	return
}

func (s *Server) Stop() {
	s.GRPC.Stop()

	close(s.QuitCh)
}

func (s *Server) InProcessClient(onServeFail ...func(error)) (conn *grpc.ClientConn, err error) {
	lsnr := bufconn.Listen(inProcessConnBufferSize)

	go func() {
		err := s.GRPC.Serve(lsnr)
		if err != nil {
			for _, f := range onServeFail {
				f(err)
			}
		}
	}()

	conn, err = grpc.Dial("",
		grpc.WithInsecure(),
		grpc.WithDialer(func(_ string, _ time.Duration) (net.Conn, error) {
			return lsnr.Dial()
		}))

	return
}
