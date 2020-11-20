package client

import (
	"github.com/google/btree"
	"google.golang.org/protobuf/proto"
)

type kv struct {
	Path  string
	Value proto.Message
}

func (kv1 kv) Less(i btree.Item) bool {
	return kv1.Path < i.(kv).Path
}
