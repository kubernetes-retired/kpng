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

package fullstate

import (
	"testing"

	"github.com/golang/protobuf/proto"

	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
)

var syncOp = &localnetv1.OpItem{Op: &localnetv1.OpItem_Sync{Sync: &localnetv1.EmptyOp{}}}

func TestAddRemoveService(t *testing.T) {
	var latestSeps []*ServiceEndpoints

	sink := New(nil)
	sink.Callback = ArrayCallback(func(seps []*ServiceEndpoints) {
		t.Logf("callback received %d seps: ", len(seps))
		for _, sep := range seps {
			t.Log("- ", sep)
		}
		latestSeps = seps
	})

	svcRef := &localnetv1.Ref{
		Set:  localnetv1.Set_LocalServicesSet,
		Path: "test/nginx",
	}
	svcBytes, _ := proto.Marshal(&localnetv1.Service{
		Namespace: "test",
		Name:      "nginx",
	})

	sink.Send(&localnetv1.OpItem{
		Op: &localnetv1.OpItem_Set{
			Set: &localnetv1.Value{
				Ref:   svcRef,
				Bytes: svcBytes,
			},
		},
	})
	sink.Send(syncOp)

	if len(latestSeps) != 1 {
		t.Fail()
	}

	sink.Send(&localnetv1.OpItem{
		Op: &localnetv1.OpItem_Delete{
			Delete: svcRef,
		},
	})
	sink.Send(syncOp)

	if len(latestSeps) != 0 {
		t.Fail()
	}
}
