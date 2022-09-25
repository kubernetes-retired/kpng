/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
