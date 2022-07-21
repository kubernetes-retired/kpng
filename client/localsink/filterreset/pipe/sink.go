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

package pipe

import (
	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client"
	"sigs.k8s.io/kpng/client/localsink"
)

type Sink struct {
	targetSinks []localsink.Sink
	buffer      []*client.ServiceEndpoints
	localsink.Config
}

func New(targetSinks ...localsink.Sink) *Sink {
	return &Sink{
		targetSinks: targetSinks,
	}
}

func (ps *Sink) Reset() {
	for _, sink := range ps.targetSinks {
		sink.Reset()
	}
}

func (ps *Sink) Setup() {
	for _, sink := range ps.targetSinks {
		sink.Setup()
	}
}

func (ps *Sink) Send(op *localnetv1.OpItem) error {
	for _, sink := range ps.targetSinks {
		if err := sink.Send(op); err != nil {
			return err
		}
	}
	return nil
}
