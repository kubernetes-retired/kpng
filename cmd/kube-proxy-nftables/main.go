package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/client"
	"k8s.io/klog"
)

var (
	dryRun       = flag.Bool("dry-run", false, "dry run (do not apply rules)")
	hookPrio     = flag.Int("hook-priority", 0, "nftable hooks priority")
	skipComments = flag.Bool("skip-comments", false, "don't comment rules")
	splitBits    = flag.Int("split-bits", 24, "dispatch services in multiple chains, spliting at the nth bit")
	splitBits6   = flag.Int("split-bits6", 120, "dispatch services in multiple chains, spliting at the nth bit (for IPv6)")

	fullResync = true
)

// FIXME atomic delete with references are currently buggy, so defer it
const deferDelete = true

// FIXME defer delete also is buggy; having to wait ~1s which is not acceptable...
const canDeleteChains = false

func init() {
	klog.InitFlags(flag.CommandLine)
}

func main() {
	client.RunWithIterator(updateNftables)
}

func updateNftables(iter *client.Iterator) {
	svcCount := 0
	epCount := 0

	{
		start := time.Now()
		defer func() {
			klog.V(1).Infof("%d services and %d endpoints applied in %v", svcCount, epCount, time.Since(start))
		}()
	}

	defer chainBuffers4.Reset()
	defer chainBuffers6.Reset()

	rule := new(bytes.Buffer)

	ipv4Mask := net.CIDRMask(*splitBits, 32)
	ipv6Mask := net.CIDRMask(*splitBits6, 128)

	chain4Nets := map[string]*net.IPNet{}
	chain6Nets := map[string]*net.IPNet{}

	for endpoints := range iter.Ch {
		// only handle cluster IPs
		if endpoints.Type != "ClusterIP" {
			continue
		}

		svcCount++

		ips := &localnetv1.IPSet{}

		if ip := endpoints.IPs.ClusterIP; ip != "" && ip != "None" {
			ips.Add(ip)
		}

		ips.AddSet(endpoints.IPs.ExternalIPs)

		for _, set := range []struct {
			ips []string
			v6  bool
		}{
			{ips.V4, false},
			{ips.V6, true},
		} {
			ips := set.ips

			if len(ips) == 0 {
				continue
			}

			familly := "ip"
			chainBuffers := chainBuffers4
			if set.v6 {
				familly = "ip6"
				chainBuffers = chainBuffers6
			}

			endpointIPs := make([]string, 0, len(endpoints.Endpoints))
			for _, ep := range endpoints.Endpoints {
				epIPs := ep.IPs.V4
				if set.v6 {
					epIPs = ep.IPs.V6
				}

				if len(epIPs) == 0 {
					continue
				}

				endpointIPs = append(endpointIPs, epIPs[0])
			}
			epCount += len(endpointIPs)

			for _, protocol := range []localnetv1.Protocol{
				localnetv1.Protocol_TCP,
				localnetv1.Protocol_UDP,
				localnetv1.Protocol_SCTP,
			} {
				rule.Reset()

				n, err := dnatRule{
					Namespace:   endpoints.Namespace,
					Name:        endpoints.Name,
					Familly:     familly,
					ServiceIPs:  ips,
					Protocol:    protocol,
					Ports:       endpoints.Ports,
					EndpointIPs: endpointIPs,
				}.WriteTo(rule)

				if err != nil {
					klog.Error("failed to write rule: ", err)
					continue
				}

				if n == 0 {
					continue
				}

				fmt.Fprintln(rule)

				// filter or nat? reject does not work in prerouting
				prefix := "dnat_"
				if len(endpointIPs) == 0 {
					prefix = "filter_"
				}

				// use ClusterIP to group rules in chains
				ip := net.ParseIP(endpoints.IPs.ClusterIP)

				mask := ipv4Mask
				if ip.To4() == nil {
					mask = ipv6Mask
				}

				ip = ip.Mask(mask)

				chain := prefix + "net_" + hex.EncodeToString(ip)

				// write rule to the chain
				rule.WriteTo(chainBuffers.Get(chain))

				// reference the chain from the dispatch if the cluster IP is of the current familly
				ipNet := &net.IPNet{
					IP:   ip,
					Mask: mask,
				}

				if set.v6 && ip.To4() == nil {
					chain6Nets[chain] = ipNet
				} else if !set.v6 && ip.To4() != nil {
					chain4Nets[chain] = ipNet
				}

				// handle external IPs dispatch
				extIPs := endpoints.IPs.ExternalIPs.V4
				if set.v6 {
					extIPs = endpoints.IPs.ExternalIPs.V6
				}

				if len(extIPs) != 0 {
					protoMatch := "protocol "
					if set.v6 {
						protoMatch = "nexthdr "
					}

					switch protocol {
					case localnetv1.Protocol_TCP:
						protoMatch += "tcp"
					case localnetv1.Protocol_UDP:
						protoMatch += "udp"
					case localnetv1.Protocol_SCTP:
						protoMatch += "sctp"
					}

					fmt.Fprintf(chainBuffers.Get(prefix+"external"), "  %s %s %s daddr {%s} goto %s\n",
						familly, protoMatch, familly, strings.Join(extIPs, ", "), chain)
				}
			}
		}
	}

	if iter.RecvErr != nil {
		fullResync = true // recv error, fully resync on next call
		return
	}

	// dispatch chains
	addDispatchChains("ip", chainBuffers4, chain4Nets)
	addDispatchChains("ip6", chainBuffers6, chain6Nets)

	// check if we have changes to apply
	if !fullResync && !chainBuffers4.Changed() && !chainBuffers6.Changed() {
		klog.V(1).Info("no changes to apply")
		return
	}

	// render the rule set
retry:
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
				klog.Infof("doing a full resync avec nft failure")
				fullResync = true
				goto retry
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

func addDispatchChains(familly string, chainBuffers *chainBufferSet, chainNets map[string]*net.IPNet) {
	chains := chainBuffers.List()
	for _, prefix := range []string{"dnat_", "filter_"} {
		chain := chainBuffers.Get("z_" + prefix + "all")

		others := make([]string, 0)
		targets := make([]string, 0)
		for _, target := range chains {
			if !strings.HasPrefix(target, prefix) {
				continue
			}

			ipNet := chainNets[target]
			if ipNet == nil {
				if !strings.HasPrefix(target, prefix+"net_") {
					// unknown chains in the prefix go to the global dispatch
					others = append(others, target)
				}
				continue
			}

			ones, _ := ipNet.Mask.Size()
			targets = append(targets, fmt.Sprintf("%s/%d: jump %s", ipNet.IP.String(), ones, target))
		}

		if len(targets) != 0 {
			fmt.Fprintf(chain, "  %s daddr vmap { \\\n    %s}\n", familly, strings.Join(targets, ", \\\n    "))
		}

		for _, other := range others {
			fmt.Fprintf(chain, "  goto %s\n", other)
		}
	}

	if chainBuffers.Get("z_dnat_all").Len() != 0 {
		fmt.Fprintf(chainBuffers.Get("hook_nat_prerouting"),
			"  type nat hook prerouting priority %d;\n  jump z_dnat_all\n", *hookPrio)
		fmt.Fprintf(chainBuffers.Get("hook_nat_output"),
			"  type nat hook output priority %d;\n  jump z_dnat_all\n", *hookPrio)
	}

	if chainBuffers.Get("z_filter_all").Len() != 0 {
		fmt.Fprintf(chainBuffers.Get("hook_filter_forward"),
			"  type filter hook forward priority %d;\n  jump z_filter_all\n", *hookPrio)
		fmt.Fprintf(chainBuffers.Get("hook_filter_output"),
			"  type filter hook output priority %d;\n  jump z_filter_all\n", *hookPrio)
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

	for _, table := range []struct {
		familly, name string
		chains        *chainBufferSet
	}{
		{"ip", "k8s_svc", chainBuffers4},
		{"ip6", "k8s_svc6", chainBuffers6},
	} {
		chains := table.chains.List()
		if fullResync {
			fmt.Fprintf(out, "table %s %s\n", table.familly, table.name)
			fmt.Fprintf(out, "delete table %s %s\n", table.familly, table.name)

		} else {
			if !table.chains.Changed() {
				continue
			}

			// flush deleted chains
			for _, chain := range table.chains.Deleted() {
				fmt.Fprintf(out, "flush chain %s %s %s\n", table.familly, table.name, chain)
			}

			// update only changed rules
			changedChains := make([]string, 0, len(chains))

			// flush changed chains
			for _, chain := range chains {
				c := table.chains.Get(chain)
				if !c.Changed() {
					continue
				}

				if !c.Created() {
					fmt.Fprintf(out, "flush chain %s %s %s\n", table.familly, table.name, chain)
				}

				changedChains = append(changedChains, chain)
			}

			chains = changedChains
		}

		// create/update changed chains
		if len(chains) != 0 {
			fmt.Fprintf(out, "table %s %s {\n", table.familly, table.name)

			for _, chain := range chains {
				c := table.chains.Get(chain)

				fmt.Fprintf(out, " chain %s {\n", chain)
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
				for _, chain := range table.chains.Deleted() {
					fmt.Fprintf(out, "delete chain %s %s %s\n", table.familly, table.name, chain)
				}
			}
		}
	}

	out.Flush()
}
