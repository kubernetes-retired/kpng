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
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"sigs.k8s.io/kpng/client"
)

var (
	extLBsOnly     = flag.Bool("load-balancers-only", false, "only manage services of type LoadBalancer")
	iptChainPrefix = flag.String("iptables-chain-prefix", "k8s-", "prefix of iptables chains")
	dryRun         = flag.Bool("dry-run", false, "dry run")
)

func main() {
	client.Run(handleEndpoints)
}

func handleEndpoints(items []*client.ServiceEndpoints) {
	// filter endpoints
	filteredItems := make([]*client.ServiceEndpoints, 0, len(items))

	for _, item := range items {
		if *extLBsOnly && item.Service.Type != "LoadBalancer" {
			// only process LBs
			continue
		}

		if len(item.Service.IPs.ExternalIPs.V4) == 0 {
			// filter out services without external IPs
			continue
		}

		filteredItems = append(filteredItems, item)
	}

	// table names
	forwardChain := *iptChainPrefix + "forward"
	dnatChain := *iptChainPrefix + "DNAT"
	snatChain := *iptChainPrefix + "SNAT"

	// build filter table
	ipt := &bytes.Buffer{}

	fmt.Fprint(ipt, "*filter\n")
	fmt.Fprint(ipt, ":", forwardChain, " -\n")
	for _, sep := range filteredItems {
		svc := sep.Service

		key := path.Join(svc.Namespace, svc.Name)
		for _, ep := range sep.Endpoints {
			for _, ip := range ep.IPs.V4 {
				for _, port := range svc.Ports {
					proto := strings.ToLower(port.Protocol.String())

					fmt.Fprintf(ipt, "-A %s -d %s -j ACCEPT -m %s -p %s --dport %d %s\n",
						forwardChain, ip, proto, proto, port.TargetPort,
						iptCommentf("%s: %s:%d -> %d", key, proto, port.Port, port.TargetPort))
				}
			}
		}
	}

	fmt.Fprint(ipt, "COMMIT\n")

	// NAT chain
	fmt.Fprint(ipt, "*nat\n")
	fmt.Fprint(ipt, ":", dnatChain, " -\n")
	fmt.Fprint(ipt, ":", snatChain, " -\n")

	// DNAT rules
	for _, sep := range filteredItems {
		svc := sep.Service

		key := path.Join(svc.Namespace, svc.Name)

		// compute target IPs
		targetIPs := make([]string, 0)

		for _, ep := range sep.Endpoints {
			if len(ep.IPs.V4) == 0 {
				continue
			}

			targetIPs = append(targetIPs, ep.IPs.V4[0])
		}

		targetCount := len(targetIPs)

		if targetCount == 0 {
			continue
		}

		for _, extIP := range svc.IPs.ExternalIPs.V4 {
			for i, ip := range targetIPs {
				rndProba := iptRandom(i, targetCount)

				for _, port := range svc.Ports {
					proto := strings.ToLower(port.Protocol.String())

					fmt.Fprintf(ipt, "-A %s -d %s -m %s -p %s --dport %d -j DNAT --to-destination %s:%d %s %s\n",
						dnatChain, extIP, proto, proto, port.Port, ip, port.TargetPort, rndProba,
						iptCommentf("%s: %s:%d -> %s:%d", key, extIP, port.Port, ip, port.TargetPort))
				}
			}
		}
	}

	// SNAT rules
	revExt := map[string]struct {
		key   string
		extIP string
	}{}
	for _, sep := range filteredItems {
		if len(sep.Endpoints) == 0 {
			continue
		}

		svc := sep.Service

		key := path.Join(svc.Namespace, svc.Name)

		// use the first external IP
		extIP := svc.IPs.ExternalIPs.V4[0]

		for _, ep := range sep.Endpoints {
			for _, ip := range ep.IPs.V4 {
				if revExt[ip].extIP == "" || extIP < revExt[ip].extIP {
					revExt[ip] = struct{ key, extIP string }{key, extIP}
				}
			}
		}
	}

	epIPs := make([]string, 0, len(revExt))
	for epIP := range revExt {
		epIPs = append(epIPs, epIP)
	}

	sort.Strings(epIPs)

	for _, epIP := range epIPs {
		rev := revExt[epIP]
		fmt.Fprintf(ipt, "-A %s -s %s -j SNAT --to-source %s %s\n",
			snatChain, epIP, rev.extIP,
			iptCommentf("%s: external IP", rev.key))
	}

	fmt.Fprint(ipt, "COMMIT\n")

	// XXX we're managing a subset only, so we may wish to reduce update load
	// newHash := xxhash.Checksum64(ipt.Bytes())
	// if prevHash == newHash {
	// 	continue
	// }

	log.Print("ext-iptables: rules have changed, updating")
	rules := ipt.Bytes()

	if *dryRun {
		log.Printf("would have applied those rules:\n%s", ipt.String())
		return
	}

	// setup iptables command
	cmd := exec.Command("iptables-restore", "--noflush")

	cmd.Stdin = ipt
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		log.Print("ext-iptables: failed to restore iptables rules: ", err, "\n", string(rules))
	}
}

func iptComment(comment string) string {
	return fmt.Sprintf("-m comment --comment %q", comment)
}

func iptCommentf(pattern string, values ...interface{}) string {
	return iptComment(fmt.Sprintf(pattern, values...))
}

func iptRandom(idx, count int) string {
	proba := 1.0 / float64(count-idx)
	if proba == 1 {
		return ""
	}
	return fmt.Sprintf(" -m statistic --mode random --probability %.4f", proba)
}
