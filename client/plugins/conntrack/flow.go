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
