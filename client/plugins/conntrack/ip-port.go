package conntrack

import (
	"strconv"
	"strings"

	api "sigs.k8s.io/kpng/api/localnetv1"
)

type IPPort struct {
	Protocol api.Protocol
	DnatIP   string
	Port     int32
}

func (i IPPort) Key() string {
	b := new(strings.Builder)
	i.writeKeyTo(b)
	return b.String()
}

func (i IPPort) writeKeyTo(b *strings.Builder) {
	b.WriteString(i.Protocol.String())
	b.WriteByte('/')
	b.WriteString(i.DnatIP)
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(int(i.Port)))
}
