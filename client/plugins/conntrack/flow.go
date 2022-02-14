package conntrack

import (
	"strconv"
	"strings"
)

type Flow struct {
	IPPort
	EndpointIP string
	TargetPort int32
}

func (f Flow) Key() string {
	b := new(strings.Builder)

	f.IPPort.writeKeyTo(b)
	b.WriteByte('>')
	b.WriteString(f.EndpointIP)
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(int(f.TargetPort)))

	return b.String()
}
