package client

import "github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"

type Iterator struct {
	Ch       <-chan *localnetv1.ServiceEndpoints
	Canceled bool
    RecvErr  error
}
