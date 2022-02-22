package serde

import (
	"testing"

	"sigs.k8s.io/kpng/api/localnetv1"
)

func TestHashIsStable(t *testing.T) {
	ep := &localnetv1.Endpoint{}
	ep.PortOverrides = map[string]int32{"a": 1, "b": 2, "c": 3}

	ref := Hash(ep)
	for i := 0; i < 100; i++ {
		h := Hash(ep)
		if ref != h {
			t.Errorf("hash is not stable: %x != %x", ref, h)
		}
	}
}
