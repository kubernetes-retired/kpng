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

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

type IPPort struct {
	IP   net.IP
	Port int32
}

func (ipp *IPPort) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%s:%d", ipp.IP, ipp.Port))
}

func (ipp *IPPort) UnmashalJSON(ba []byte) (err error) {
	s := ""
	if err = json.Unmarshal(ba, &s); err != nil {
		return
	}

	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return
	}

	p, err := strconv.Atoi(port)
	if err != nil {
		return
	}

	ipp.IP = net.ParseIP(host)
	ipp.Port = int32(p)

	return
}
