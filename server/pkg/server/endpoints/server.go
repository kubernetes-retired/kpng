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

package endpoints

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"

	"k8s.io/klog"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/server/jobs/store2localdiff"
	"sigs.k8s.io/kpng/server/pkg/proxystore"
)

type Server struct {
	Store *proxystore.Store
}

var syncItem = &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{}}

func (s *Server) Watch(res localnetv1.Endpoints_WatchServer) error {
	remote := ""
	{
		ctxPeer, _ := peer.FromContext(res.Context())
		remote = ctxPeer.Addr.String()
	}

	klog.Info("new connection from ", remote)
	defer klog.Info("connection from ", remote, " closed")

	job := &store2localdiff.Job{
		Store: s.Store,
		Sink:  serverSink{res, remote},
	}

	return job.Run(res.Context())
}

type serverSink struct {
	localnetv1.Endpoints_WatchServer
	remote string
}

func (s serverSink) Setup() { /* noop */ }

func (s serverSink) WaitRequest() (nodeName string, err error) {
	req, err := s.Recv()

	if err != nil {
		err = grpc.Errorf(codes.Aborted, "recv error: %v", err)
		return
	}

	klog.V(1).Info("remote ", s.remote, " requested node ", req.NodeName)

	nodeName = req.NodeName
	return
}

func (s serverSink) Reset() {}
