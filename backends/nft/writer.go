package nft

import (
	"bytes"
	"io"
)

type writer interface {
	io.Writer
	WriteByte(b byte) (err error)
	WriteString(s string) (n int, err error)
}

var (
	_ writer = new(bytes.Buffer)
	_ writer = new(Leaf)
)
