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
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"sigs.k8s.io/kpng/client/localsink/fullstate"
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

func Callback(ch <-chan *fullstate.ServiceEndpoints) {
	svcCount := 0
	epCount := 0

	start := time.Now()
	defer func() {
		klog.V(1).Infof("%d services and %d endpoints applied in %v", svcCount, epCount, time.Since(start))
	}()

	defer table4.Reset()
	defer table6.Reset()

	renderContexts := []*renderContext{
		newRenderContext(table4, clusterCIDRsV4, net.CIDRMask(*splitBits, 32)),
		newRenderContext(table6, clusterCIDRsV6, net.CIDRMask(*splitBits6, 128)),
	}

	for serviceEndpoints := range ch {
		// types we don't handle
		switch serviceEndpoints.Service.Type {
		case "ExternalName":
			continue
		}

		svcCount++

		for _, ctx := range renderContexts {
			ctx.addServiceEndpoints(serviceEndpoints)
			epCount += ctx.epCount
		}
	}

	for _, ctx := range renderContexts {
		ctx.Finalize()
	}

	// check if we have changes to apply
	if !fullResync && !table4.Changed() && !table6.Changed() {
		klog.V(1).Info("no changes to apply")
		return
	}

	klog.V(1).Infof("nft rules generated (%s)", time.Since(start))

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

			if klog.V(2).Enabled() {
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
	dnatAll := table.Chains.Get("z_dnat_all")
	if *withTrace {
		dnatAll.WriteString("  meta nftrace set 1\n")
	}

	// DNAT
	if table.Chains.Has("z_dispatch_svc_dnat") {
		fmt.Fprint(dnatAll, "  jump z_dispatch_svc_dnat\n")
	}

	if table.Chains.Has("dnat_external") {
		fmt.Fprint(dnatAll, "  jump dnat_external\n")
	}

	if table.Chains.Has("nodeports_dnat") {
		dnatAll.WriteString("  fib daddr type local jump nodeports_dnat\n")
	}

	if dnatAll.Len() != 0 {
		for _, hook := range []string{"prerouting", "output"} {
			fmt.Fprintf(table.Chains.Get("z_hook_nat_"+hook),
				"  type nat hook "+hook+" priority %d;\n  jump z_dnat_all\n", *hookPrio)
		}
	}

	// filtering
	filterAll := table.Chains.Get("z_filter_all")
	fmt.Fprint(filterAll, "  ct state invalid drop\n")

	if table.Chains.Has("z_dispatch_svc_filter") {
		fmt.Fprint(filterAll, "  jump z_dispatch_svc_filter\n")
	}

	if table.Chains.Has("filter_external") {
		fmt.Fprint(filterAll, "  jump filter_external\n")
	}

	if table.Chains.Has("nodeports_filter") {
		filterAll.WriteString("  fib daddr type local jump nodeports_filter\n")
	}

	fmt.Fprintf(table.Chains.Get("z_hook_filter_forward"),
		"  type filter hook forward priority %d;\n  jump z_filter_all\n", *hookPrio)
	fmt.Fprintf(table.Chains.Get("z_hook_filter_output"),
		"  type filter hook output priority %d;\n  jump z_filter_all\n", *hookPrio)
}

func addPostroutingChain(table *nftable, clusterCIDRs []string, localEndpointIPs []string) {
	hasCIDRs := len(clusterCIDRs) != 0
	hasLocalEPs := len(localEndpointIPs) != 0

	if !hasCIDRs && !hasLocalEPs {
		return
	}

	chain := table.Chains.Get("zz_hook_nat_postrouting")
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
		fmt.Fprint(chain, "  fib daddr type != local \\\n")
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

	if klog.V(2).Enabled() {
		outputs = append(outputs, os.Stdout)
	}

	out := bufio.NewWriter(io.MultiWriter(outputs...))

	for _, table := range allTables {
		// flush/delete previous state
		if fullResync {
			fmt.Fprintf(out, "table %s %s\n", table.Family, table.Name)
			fmt.Fprintf(out, "delete table %s %s\n", table.Family, table.Name)

		} else {
			for _, ks := range table.KindStores() {
				// flush deleted elements
				for _, item := range ks.Store.Deleted() {
					fmt.Fprintf(out, "flush %s %s %s %s\n", ks.Kind, table.Family, table.Name, item.Key())
				}

				// flush changed elements
				for _, item := range ks.Store.Changed() {
					if item.Created() {
						continue
					}
					fmt.Fprintf(out, "flush %s %s %s %s\n", ks.Kind, table.Family, table.Name, item.Key())
				}
			}
		}

		// create/update changed elements
		fmt.Fprintf(out, "table %s %s {\n", table.Family, table.Name)
		for _, ki := range table.OrderedChanges(fullResync) {
			fmt.Fprintf(out, " %s %s {\n", ki.Kind, ki.Item.Key())
			io.Copy(out, ki.Item.Value())
			fmt.Fprintln(out, " }")
		}
		fmt.Fprintln(out, "}")

		// delete removed elements (already done by deleting the table on fullResync)
		if !fullResync {
			// delete
			if canDeleteChains {
				var out io.Writer = out
				if deferDelete {
					out = deferred
				}
				for _, ks := range table.KindStores() {
					for _, item := range ks.Store.Deleted() {
						fmt.Fprintf(out, "delete %s %s %s %s\n", ks.Kind, table.Family, table.Name, item.Key())
					}
				}
			}
		}
	}

	out.Flush()
}

// nftKey convert an expected key to the real key to write to nft. It should be the same but some nft versions have a bug.
func nftKey(x int) (y int) {
	if hasNFTHashBug {
		return (x>>0&0xff)<<24 |
			(x>>8&0xff)<<(24-8) |
			(x>>16&0xff)<<(24-16)
	}
	return x
}
