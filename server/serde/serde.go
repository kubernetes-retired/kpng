package serde

import (
	"github.com/cespare/xxhash"
	"google.golang.org/protobuf/proto"
)

var (
	// Endpoints have maps => request deterministic marshalling
	marshalOpts = proto.MarshalOptions{Deterministic: true}
)

func Marshal(m proto.Message) []byte {
	ba, err := marshalOpts.Marshal(m)
	if err != nil {
		panic(err) // unexpected
	}
	return ba
}

func Hash(m proto.Message) uint64 {
	return xxhash.Sum64(Marshal(m))
}
