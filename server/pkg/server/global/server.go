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
	"sigs.k8s.io/kpng/api/localv1"
	"sigs.k8s.io/kpng/api/globalv1"
	"sigs.k8s.io/kpng/server/jobs/store2globaldiff"
	"sigs.k8s.io/kpng/server/proxystore"
)

type Server struct {
	globalv1.UnimplementedSetsServer

	Store *proxystore.Store
}

var syncItem = &localv1.OpItem{Op: &localv1.OpItem_Sync{}}

func (s *Server) Watch(res globalv1.Sets_WatchServer) error {
	w := resWrap{res}

	job := &store2globaldiff.Job{
		Store: s.Store,
		Sink:  w,
	}

	return job.Run(res.Context())
}

type resWrap struct {
	globalv1.Sets_WatchServer
}

func (w resWrap) Wait() error {
	_, err := w.Recv()
	return err
}
