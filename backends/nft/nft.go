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

package nft

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	localnetv1 "sigs.k8s.io/kpng/api/localnetv1"
	"strconv"
	"strings"
	"time"

	"github.com/cespare/xxhash"
	"github.com/spf13/pflag"
	"k8s.io/klog"

	"sigs.k8s.io/kpng/client"
)

var (
	flag = &pflag.FlagSet{}

	dryRun          = flag.Bool("dry-run", false, "dry run (do not apply rules)")
	hookPrio        = flag.Int("hook-priority", 0, "nftable hooks priority")
	skipComments    = flag.Bool("skip-comments", false, "don't comment rules")
	splitBits       = flag.Int("split-bits", 24, "dispatch services in multiple chains, spliting at the nth bit")
	splitBits6      = flag.Int("split-bits6", 120, "dispatch services in multiple chains, spliting at the nth bit (for IPv6)")
	mapsCount       = flag.Uint64("maps-count", 0x100, "number of endpoints maps to use")
	forceNFTHashBug = flag.Bool("force-nft-hash-workaround", false, "bypass auto-detection of NFT hash bug (necessary when nft is blind)")
	withTrace       = flag.Bool("trace", false, "enable nft trace")

	clusterCIDRsFlag = flag.StringSlice("cluster-cidrs", []string{"0.0.0.0/0"}, "cluster IPs CIDR that shoud not be masqueraded")
	clusterCIDRsV4   []string
	clusterCIDRsV6   []string

	fullResync = true

	hasNFTHashBug = false
)

func BindFlags(flags *pflag.FlagSet) {
	flags.AddFlagSet(flag)
}

// FIXME atomic delete with references are currently buggy, so defer it
const deferDelete = true

// FIXME defer delete also is buggy; having to wait ~1s which is not acceptable...
const canDeleteChains = false

func PreRun() {
	checkIPTableVersion()
	checkMapIndexBug()

	// parse cluster CIDRs
	clusterCIDRsV4 = make([]string, 0)
	clusterCIDRsV6 = make([]string, 0)
	for _, cidr := range *clusterCIDRsFlag {
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			klog.Fatalf("bad CIDR given: %q: %v", cidr, err)
		}

		if ip.To4() == nil {
			clusterCIDRsV6 = append(clusterCIDRsV6, ipNet.String())
		} else {
			clusterCIDRsV4 = append(clusterCIDRsV4, ipNet.String())
		}
	}

	klog.Info("cluster CIDRs V4: ", clusterCIDRsV4)
	klog.Info("cluster CIDRs V6: ", clusterCIDRsV6)
}

func Callback(ch <-chan *client.ServiceEndpoints) {
	svcCount := 0
	epCount := 0
	doComments := !*skipComments && bool(klog.V(1))

	{
		start := time.Now()
		defer func() {
			klog.V(1).Infof("%d services and %d endpoints applied in %v", svcCount, epCount, time.Since(start))
		}()
	}

	defer table4.Reset()
	defer table6.Reset()

	rule := new(bytes.Buffer)

	ipv4Mask := net.CIDRMask(*splitBits, 32)
	ipv6Mask := net.CIDRMask(*splitBits6, 128)

	chain4Nets := map[string]bool{}
	chain6Nets := map[string]bool{}

	mapOffsets := make([]uint64, *mapsCount)

	epSeen := map[string]bool{}
	localEndpointIPsV4 := make([]string, 0, 256)
	localEndpointIPsV6 := make([]string, 0, 256)

	for serviceEndpoints := range ch {
		svc := serviceEndpoints.Service
		endpoints := serviceEndpoints.Endpoints

		// types we don't handle
		if svc.Type == "ExternalName" {
			continue
		}

		mapH := xxhash.Sum64String(svc.Namespace+"/"+svc.Name) % (*mapsCount)
		svcOffset := mapOffsets[mapH]
		mapOffsets[mapH] += uint64(len(endpoints))

		endpointsMap := fmt.Sprintf("endpoints_%04x", mapH)

		svcCount++

		clusterIPs := &localnetv1.IPSet{}
		ips := &localnetv1.IPSet{}

		if svc.IPs.ClusterIPs != nil {
			clusterIPs.AddSet(svc.IPs.ClusterIPs)
			ips.AddSet(svc.IPs.ClusterIPs)
		}
		ips.AddSet(svc.IPs.ExternalIPs)

		for _, set := range []struct {
			ips              []string
			v6               bool
			localEndpointIPs *[]string
		}{
			{ips.V4, false, &localEndpointIPsV4},
			{ips.V6, true, &localEndpointIPsV6},
		} {
			ips := set.ips

			if len(ips) == 0 {
				continue
			}

			family := "ip"
			chainBuffers := table4
			if set.v6 {
				family = "ip6"
				chainBuffers = table6
			}

			// compute endpoints
			endpointIPs := make([]string, 0, len(endpoints))
			for _, ep := range endpoints {
				epIPs := ep.IPs.V4
				if set.v6 {
					epIPs = ep.IPs.V6
				}

				if len(epIPs) == 0 {
					continue
				}

				endpointIPs = append(endpointIPs, epIPs[0])

				if ep.Local {
					for _, ip := range epIPs {
						if !epSeen[ip] {
							epSeen[ip] = true
							*set.localEndpointIPs = append(*set.localEndpointIPs, ip)
						}
					}
				}
			}
			epCount += len(endpointIPs)

			// add endpoints to the map
			if len(endpointIPs) != 0 {
				epMap := chainBuffers.Get("map", endpointsMap)
				if epMap.Len() == 0 {
					epMap.WriteString("  typeof numgen random mod 1 : ")
					epMap.WriteString(family)
					epMap.WriteString(" daddr\n")
					epMap.WriteString("  elements = {")
					epMap.Defer(func(m *chainBuffer) { m.WriteString("}\n") })
				} else {
					epMap.WriteString(", ")
				}

				if doComments {
					fmt.Fprintf(epMap, "\\\n    # %s/%s", svc.Namespace, svc.Name)
				}

				fmt.Fprint(epMap, "\\\n    ")
				for idx, ip := range endpointIPs {
					if idx != 0 {
						epMap.Write([]byte{',', ' '})
					}
					key := svcOffset + uint64(idx)

					if hasNFTHashBug {
						key = 0 |
							(key>>0&0xff)<<24 |
							(key>>8&0xff)<<(24-8) |
							(key>>16&0xff)<<(24-16) |
							0
					}

					epMap.WriteString(strconv.FormatUint(key, 10))
					epMap.WriteString(" : ")
					epMap.WriteString(ip)
				}
			}

			// filter or nat? reject does not work in prerouting
			prefix := "dnat_"
			if len(endpointIPs) == 0 {
				prefix = "filter_"
			}

			daddrMatch := family + " daddr"

			svc_chain := prefix + strings.Join([]string{"svc", svc.Namespace, svc.Name}, "_")

			hasRules := false
			for _, protocol := range []localnetv1.Protocol{
				localnetv1.Protocol_TCP,
				localnetv1.Protocol_UDP,
				localnetv1.Protocol_SCTP,
			} {
				rule.Reset()

				// build the rule
				ruleSpec := dnatRule{
					Namespace:   svc.Namespace,
					Name:        svc.Name,
					Protocol:    protocol,
					Ports:       svc.Ports,
					EndpointIPs: endpointIPs,
				}
				ruleSpec.WriteTo(rule, false, endpointsMap, svcOffset)

				if rule.Len() == 0 {
					continue
				}

				rule.WriteTo(chainBuffers.Get("chain", svc_chain))
				hasRules = true

				// hande node ports
				rule.Reset()
				ruleSpec.WriteTo(rule, true, endpointsMap, svcOffset)

				if rule.Len() == 0 {
					continue
				}

				rule.WriteTo(chainBuffers.Get("chain", "nodeports"))
			}

			if !hasRules {
				continue
			}

			// dispatch group chain (ie: dnat_net_0a002700 for 10.0.39.x and a /24 mask)
			familyClusterIPs := clusterIPs.V4
			if set.v6 {
				familyClusterIPs = clusterIPs.V6
			}

			if len(familyClusterIPs) != 0 {
				// this family owns the cluster IP => build the dispatch chain
				mask := ipv4Mask
				if set.v6 {
					mask = ipv6Mask
				}

				for _, ipStr := range familyClusterIPs {
					ip := net.ParseIP(ipStr).Mask(mask)

					// get the dispatch chain
					chain := prefix + "net_" + hex.EncodeToString(ip)

					// add service chain in dispatch
					vmapAdd(chainBuffers.Get("chain", chain), family+" daddr", fmt.Sprintf("%s: jump %s", ipStr, svc_chain))

					// reference the dispatch chain from the global dispatch (of not already done) (ie: z_dnat_all)
					if set.v6 && !chain6Nets[chain] || !set.v6 && !chain4Nets[chain] {
						ipNet := &net.IPNet{
							IP:   ip,
							Mask: mask,
						}

						vmapAdd(chainBuffers.Get("chain", "z_"+prefix+"all"), daddrMatch, ipNet.String()+": jump "+chain)

						if set.v6 {
							chain6Nets[chain] = true
						} else {
							chain4Nets[chain] = true
						}
					}
				}
			}

			// handle external IPs dispatch
			extIPs := svc.IPs.ExternalIPs.V4
			if set.v6 {
				extIPs = svc.IPs.ExternalIPs.V6
			}

			if len(extIPs) != 0 {
				extChain := chainBuffers.Get("chain", prefix+"external")
				for _, extIP := range extIPs {
					// XXX should this be by protocol and port to allow external IP mutualization between services?
					vmapAdd(extChain, daddrMatch, extIP+": jump "+svc_chain)
				}
			}

		}
	}

	// run deferred actions
	table4.RunDeferred()
	table6.RunDeferred()

	// dispatch chains
	addDispatchChains(table4)
	addDispatchChains(table6)

	// postrouting chains
	addPostroutingChain(table4, clusterCIDRsV4, localEndpointIPsV4)
	addPostroutingChain(table6, clusterCIDRsV6, localEndpointIPsV6)

	// check if we have changes to apply
	if !fullResync && !table4.Changed() && !table6.Changed() {
		klog.V(1).Info("no changes to apply")
		return
	}

	// render the rule set
	//retry:
	cmdIn, pipeOut := io.Pipe()

	deferred := new(bytes.Buffer)
	go renderNftables(pipeOut, deferred)

	if *dryRun {
		io.Copy(ioutil.Discard, cmdIn)
		klog.Info("not running nft (dry run mode)")
	} else {
		cmd := exec.Command("nft", "-f", "-")
		cmd.Stdin = cmdIn
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		start := time.Now()
		err := cmd.Run()
		elapsed := time.Since(start)

		if err != nil {
			klog.Errorf("nft failed: %v (%s)", err, elapsed)

			// ensure render is finished
			io.Copy(ioutil.Discard, cmdIn)

			if !fullResync {
				// failsafe: rebuild everything
				klog.Infof("doing a full resync after nft failure")
				fullResync = true
				//goto retry
			}
			return
		}

		klog.V(1).Infof("nft ok (%s)", elapsed)

		if deferred.Len() != 0 {
			klog.V(1).Infof("running deferred nft actions")

			// too fast and deletes fail... :(
			//time.Sleep(100 * time.Millisecond)

			if klog.V(2) {
				os.Stdout.Write(deferred.Bytes())
			}

			cmd := exec.Command("nft", "-f", "-")
			cmd.Stdin = deferred
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err = cmd.Run()
			if err != nil {
				klog.Warning("nft deferred script failed: ", err)
			}
		}
	}

	if fullResync {
		// all done, we can valide the first run
		fullResync = false
	}
}

func addDispatchChains(table *nftable) {
	dnatAll := table.Get("chain", "z_dnat_all")
	if *withTrace {
		dnatAll.WriteString("  meta nftrace set 1\n")
	}

	// DNAT
	if table.Get("chain", "dnat_external").Len() != 0 {
		fmt.Fprint(dnatAll, "  jump dnat_external\n")
	}

	if table.Get("chain", "nodeports").Len() != 0 {
		dnatAll.WriteString("  fib daddr type local jump nodeports\n")
	}

	if dnatAll.Len() != 0 {
		for _, hook := range []string{"prerouting", "output"} {
			fmt.Fprintf(table.Get("chain", "hook_nat_"+hook),
				"  type nat hook "+hook+" priority %d;\n  jump z_dnat_all\n", *hookPrio)
		}
	}

	// filtering
	if table.Get("chain", "filter_external").Len() != 0 {
		fmt.Fprint(table.Get("chain", "z_filter_all"), "  jump filter_external\n")
	}
	if table.Get("chain", "z_filter_all").Len() != 0 {
		fmt.Fprintf(table.Get("chain", "hook_filter_forward"),
			"  type filter hook forward priority %d;\n  jump z_filter_all\n", *hookPrio)
		fmt.Fprintf(table.Get("chain", "hook_filter_output"),
			"  type filter hook output priority %d;\n  jump z_filter_all\n", *hookPrio)
	}
}

func addPostroutingChain(table *nftable, clusterCIDRs []string, localEndpointIPs []string) {
	hasCIDRs := len(clusterCIDRs) != 0
	hasLocalEPs := len(localEndpointIPs) != 0

	if !hasCIDRs && !hasLocalEPs {
		return
	}

	chain := table.Get("chain", "hook_nat_postrouting")
	fmt.Fprintf(chain, "  type nat hook postrouting priority %d;\n", *hookPrio)
	if hasCIDRs {
		chain.Writeln()
		if !*skipComments {
			fmt.Fprint(chain, "  # masquerade non-cluster traffic to non-local endpoints\n")
		}
		fmt.Fprint(chain, "  ", table.Family, " saddr != { ", strings.Join(clusterCIDRs, ", "), " } \\\n")
		if hasLocalEPs {
			fmt.Fprint(chain, "  ", table.Family, " daddr != { ", strings.Join(localEndpointIPs, ", "), " } \\\n")
		}
		fmt.Fprint(chain, "  masquerade\n")
	}

	if hasLocalEPs {
		chain.Writeln()
		if !*skipComments {
			fmt.Fprint(chain, "  # masquerade hairpin traffic\n")
		}
		chain.WriteString("  ")
		chain.WriteString(table.Family)
		chain.WriteString(" saddr . ")
		chain.WriteString(table.Family)
		chain.WriteString(" daddr { ")
		for i, ip := range localEndpointIPs {
			if i != 0 {
				chain.WriteString(", ")
			}
			chain.WriteString(ip + " . " + ip)
		}
		chain.WriteString(" } masquerade\n")
	}
}

func renderNftables(output io.WriteCloser, deferred io.Writer) {
	defer output.Close()

	outputs := make([]io.Writer, 0, 2)
	outputs = append(outputs, output)

	if klog.V(2) {
		outputs = append(outputs, os.Stdout)
	}

	out := bufio.NewWriter(io.MultiWriter(outputs...))

	for _, table := range []*nftable{table4, table6} {
		chains := table.List()

		if fullResync {
			fmt.Fprintf(out, "table %s %s\n", table.Family, table.Name)
			fmt.Fprintf(out, "delete table %s %s\n", table.Family, table.Name)

		} else {
			if !table.Changed() {
				continue
			}

			// flush deleted elements
			for _, chain := range table.Deleted() {
				fmt.Fprintf(out, "flush %s %s %s %s\n", chain.kind, table.Family, table.Name, chain.name)
			}

			// update only changed rules
			changedChains := make([]string, 0, len(chains))

			// flush changed chains
			for _, chain := range chains {
				c := table.Get("", chain)
				if !c.Changed() {
					continue
				}

				if !c.Created() {
					fmt.Fprintf(out, "flush %s %s %s %s\n", c.kind, table.Family, table.Name, chain)
				}

				changedChains = append(changedChains, chain)
			}

			chains = changedChains
		}

		// create/update changed chains
		if len(chains) != 0 {
			fmt.Fprintf(out, "table %s %s {\n", table.Family, table.Name)
			for _, chain := range chains {
				c := table.Get("", chain)

				fmt.Fprintf(out, " %s %s {\n", c.kind, chain)
				io.Copy(out, c)
				fmt.Fprintln(out, " }")
			}

			fmt.Fprintln(out, "}")
		}

		// delete removed chains (already done by deleting the table on fullResync)
		if !fullResync {
			// delete
			if canDeleteChains {
				var out io.Writer = out
				if deferDelete {
					out = deferred
				}
				for _, chain := range table.Deleted() {
					fmt.Fprintf(out, "delete %s %s %s %s\n", chain.kind, table.Family, table.Name, chain.name)
				}
			}
		}
	}

	out.Flush()
}
