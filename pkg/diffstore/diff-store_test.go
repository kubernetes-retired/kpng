package diffstore

import (
	"testing"
)

func TestStoreUpdates(t *testing.T) {
	s := New()

	tx := s.Begin([]byte("hello"))
	tx.AddJSON("alice", []byte("alice value"))
	tx.AddJSON("bob", []byte("bob value"))
	changes := tx.Apply()

	if len(changes.Set) != 2 || len(changes.Del) != 0 {
		t.Error(changes.String())
	}

	tx = s.Begin([]byte("helloo"))
	tx.AddJSON("charlie", []byte("charlie value"))
	tx.Apply()

	tx = s.Begin([]byte("hello"))
	tx.AddJSON("alice", []byte("alice value"))
	changes = tx.Apply()

	if len(changes.Set) != 0 || len(changes.Del) != 1 {
		t.Error(changes.String())
	}

	changes = s.Delete([]byte("hello"))

	if len(changes.Set) != 0 || len(changes.Del) != 1 {
		t.Error(changes.String())
	}
}
