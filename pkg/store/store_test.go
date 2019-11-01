package store

import (
	"testing"

	"k8s.io/kube-localnet-api/pkg/api/localnetv1"
)

func TestNext(t *testing.T) {
	s := New()

	s.Set([]byte("a"), &localnetv1.ServiceEndpoints{
		Name: "a",
	})

	snap := s.Next(0)
	t.Log(snap)
}
