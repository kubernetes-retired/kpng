package fullstate

import (
	"testing"

	"github.com/golang/protobuf/proto"

	localnetv12 "sigs.k8s.io/kpng/api/localnetv1"
)

var syncOp = &localnetv12.OpItem{Op: &localnetv12.OpItem_Sync{Sync: &localnetv12.EmptyOp{}}}

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

	svcRef := &localnetv12.Ref{
		Set:  localnetv12.Set_ServicesSet,
		Path: "test/nginx",
	}
	svcBytes, _ := proto.Marshal(&localnetv12.Service{
		Namespace: "test",
		Name:      "nginx",
	})

	sink.Send(&localnetv12.OpItem{
		Op: &localnetv12.OpItem_Set{
			Set: &localnetv12.Value{
				Ref:   svcRef,
				Bytes: svcBytes,
			},
		},
	})
	sink.Send(syncOp)

	if len(latestSeps) != 1 {
		t.Fail()
	}

	sink.Send(&localnetv12.OpItem{
		Op: &localnetv12.OpItem_Delete{
			Delete: svcRef,
		},
	})
	sink.Send(syncOp)

	if len(latestSeps) != 0 {
		t.Fail()
	}
}
