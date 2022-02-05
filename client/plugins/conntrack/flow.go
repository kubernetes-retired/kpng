package conntrack

import (
	"strconv"
	"strings"

	api "sigs.k8s.io/kpng/api/localnetv1"
)

type Flow struct {
	Protocol   api.Protocol
	DnatIP     string
	EndpointIP string
	Port       int32
	TargetPort int32
}

func (f Flow) Key() string {
	b := new(strings.Builder)

	b.WriteString(f.Protocol.String())
	b.WriteByte('/')
	b.WriteString(f.DnatIP)
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(int(f.Port)))
	b.WriteByte('>')
	b.WriteString(f.EndpointIP)
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(int(f.TargetPort)))

	return b.String()
}
