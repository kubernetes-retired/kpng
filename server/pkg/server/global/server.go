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

package global

import (
	"k8s.io/klog/v2"
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/server/jobs/store2globaldiff"
	"sigs.k8s.io/kpng/server/proxystore"
)

type Server struct {
	localnetv1.UnimplementedGlobalServer

	Store *proxystore.Store
}

var syncItem = &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{}}

func (s *Server) Watch(res localnetv1.Global_WatchServer) error {

	w := resWrap{res}

	klog.V(1).Info("Running global server job: watch store = %v,  sink = %v", s.Store, w)
	job := &store2globaldiff.Job{
		Store: s.Store,
		Sink:  w,
	}

	return job.Run(res.Context())
}

type resWrap struct {
	localnetv1.Global_WatchServer
}

func (w resWrap) Wait() error {
	klog.V(1).Info("global server, Running Wait()")
	_, err := w.Recv()
	return err
}
