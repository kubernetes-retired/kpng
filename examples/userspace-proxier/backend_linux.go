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
	"flag"
	"log"
	"net"
	"os/exec"

	"sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/serviceevents"
)

// Linux-specific IP management

var dummyIface string

func (b *userspaceBackend) BindFlags() {
	flag.StringVar(&dummyIface, "interface", "kpng-dummy", "interface used to hold service IPs; set to empty to disable IP management")
}

// Setup see localsink.Sink#Setup
func (b *userspaceBackend) Setup() {
	if dummyIface == "" {
		return
	}

	// create dummy interface if it doesn't exist
	if _, err := net.InterfaceByName(dummyIface); err != nil {
		log.Print("creating dummy interface ", dummyIface)

		out, err := exec.Command("ip", "link", "add", dummyIface, "type", "dummy").CombinedOutput()
		if err != nil {
			log.Fatal("failed to add interface: ", err, "\n", string(out))
		}

		out, err = exec.Command("ip", "link", "set", dummyIface, "up").CombinedOutput()
		if err != nil {
			log.Fatal("failed to set interface up: ", err, "\n", string(out))
		}
	}

	iface, err := net.InterfaceByName(dummyIface)
	if err != nil {
		log.Fatal("failed to get interface ", dummyIface, ": ", err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		log.Fatal("failed to get interface IPs: ", err)
	}

	// remove existing IPs from the interface
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		if ip.IsLinkLocalUnicast() {
			continue
		}
		if ip.IsLinkLocalMulticast() {
			continue
		}

		log.Print("removing IP ", addr)
		out, err := exec.Command("ip", "addr", "del", addr.String(), "dev", dummyIface).CombinedOutput()
		if err != nil {
			log.Print("failed to remove IP ", addr.String(), " from interface: ", err, "\n", string(out))
		}
	}
}

var _ serviceevents.IPsListener = &userspaceBackend{}

func (b *userspaceBackend) AddIP(svc *localnetv1.Service, ip string, _ serviceevents.IPKind) {
	if dummyIface == "" {
		return
	}

	if b.ips[ip] {
		return
	}

	ipMask := ip
	if net.ParseIP(ip).To4() == nil {
		ipMask += "/128"
	} else {
		ipMask += "/32"
	}

	log.Print("adding IP ", ipMask, " to ", dummyIface)
	out, err := exec.Command("ip", "addr", "add", ipMask, "dev", dummyIface).CombinedOutput()
	if err != nil {
		log.Print("warning: ignoring error while adding IP ", ipMask, " to interface ", dummyIface, ": ", err, "\n", string(out))
	}

	b.ips[ip] = true
}

func (b *userspaceBackend) DeleteIP(svc *localnetv1.Service, ip string, _ serviceevents.IPKind) {
	if dummyIface == "" {
		return
	}

	if b.ips[ip] {
		return
	}

	ipMask := ip
	if net.ParseIP(ip).To4() == nil {
		ipMask += "/128"
	} else {
		ipMask += "/32"
	}

	log.Print("removing IP ", ipMask, " from ", dummyIface)
	out, err := exec.Command("ip", "addr", "del", ipMask, "dev", dummyIface).CombinedOutput()
	if err != nil {
		log.Print("warning: ignoring error while removing IP ", ipMask, " from interface ", dummyIface, ": ", err, "\n", string(out))
	}

	b.ips[ip] = true
}
