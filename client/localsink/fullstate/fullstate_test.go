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
		Set:  localnetv1.Set_ServicesSet,
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
