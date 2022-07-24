package localnetv1

import (
	"testing"

	"github.com/golang/protobuf/proto"
)

func TestHashService(t *testing.T) {
	svc := &Service{
		Name: "svc",
		SessionAffinity: &Service_ClientIP{
			ClientIP: &ClientIPAffinity{
				TimeoutSeconds: 1,
			},
		},
	}

	ba, err := proto.Marshal(svc)
	if err != nil {
		t.Error(err)
	}

	err = proto.Unmarshal(ba, &Service{})
	if err != nil {
		t.Error(err)
	}
}
