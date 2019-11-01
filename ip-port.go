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
