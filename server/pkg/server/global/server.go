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
	store2globaldiff2 "sigs.k8s.io/kpng/server/jobs/store2globaldiff"
	localnetv12 "sigs.k8s.io/kpng/server/pkg/api/localnetv1"
	proxystore2 "sigs.k8s.io/kpng/server/pkg/proxystore"
)

type Server struct {
	Store *proxystore2.Store
}

var syncItem = &localnetv12.OpItem{Op: &localnetv12.OpItem_Sync{}}

func (s *Server) Watch(res localnetv12.Global_WatchServer) error {
	w := resWrap{res}

	job := &store2globaldiff2.Job{
		Store: s.Store,
		Sink:  w,
	}

	return job.Run(res.Context())
}

type resWrap struct {
	localnetv12.Global_WatchServer
}

func (w resWrap) Wait() error {
	_, err := w.Recv()
	return err
}
