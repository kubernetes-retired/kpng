package nft

import (
	"fmt"
	"io"
	"net"
	"os"

	v1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

func testValues() (ctx *renderContext, seps *fullstate.ServiceEndpoints) {
	table4 := newNftable("ip", "k8s_svc") // one table per test
	ctx = newRenderContext(table4, []string{"10.1.0.0/16"}, net.CIDRMask(24, 32))

	svc := &v1.Service{
		Namespace: "my-ns",
		Name:      "my-svc",
		Type:      "ClusterIP",
		IPs: &v1.ServiceIPs{
			ClusterIPs: v1.NewIPSet("10.0.0.1"),
		},
		Ports: []*v1.PortMapping{
			{Name: "http", Protocol: v1.Protocol_TCP, Port: 80, TargetPort: 8080, NodePort: 58080},
			{Name: "metrics", TargetPortName: "x", Protocol: v1.Protocol_TCP, Port: 81},
			{Name: "nowhere", TargetPortName: "y", Protocol: v1.Protocol_TCP, Port: 82, NodePort: 58081},
		},
	}

	seps = &fullstate.ServiceEndpoints{
		Service: svc,
		Endpoints: []*v1.Endpoint{
			{IPs: v1.NewIPSet("10.1.0.1"), Local: true},
			{IPs: v1.NewIPSet("10.1.0.2"), Local: true, PortOverrides: map[string]int32{"metrics": 1011}},
			{IPs: v1.NewIPSet("10.1.1.1"), Local: false, PortOverrides: map[string]int32{"metrics": 1042}},
		},
	}

	return
}

func ExampleRenderBasicService() {
	ctx, seps := testValues()

	ctx.addServiceEndpoints(seps)

	finalizeAndPrintTable(os.Stdout, ctx)

	// Output:
	// table ip k8s_svc {
	//  chain nodeports_dnat {
	//   tcp dport 58080 jump svc_my-ns_my-svc_dnat
	//  }
	//  chain nodeports_filter {
	//   tcp dport 58081 jump svc_my-ns_my-svc_filter
	//  }
	//  chain svc_my-ns_my-svc_dnat {
	//   tcp dport 80 jump svc_my-ns_my-svc_eps
	//   fib daddr type local tcp dport 58080 jump svc_my-ns_my-svc_eps
	//   tcp dport 81 jump svc_my-ns_my-svc_eps_metrics
	//  }
	//  chain svc_my-ns_my-svc_ep_0a010001 {
	//   tcp dport 80 dnat to 10.1.0.1:8080
	//   fib daddr type local tcp dport 58080 dnat to 10.1.0.1:8080
	//  }
	//  chain svc_my-ns_my-svc_ep_0a010002 {
	//   tcp dport 80 dnat to 10.1.0.2:8080
	//   tcp dport 81 dnat to 10.1.0.2:1011
	//   fib daddr type local tcp dport 58080 dnat to 10.1.0.2:8080
	//  }
	//  chain svc_my-ns_my-svc_ep_0a010101 {
	//   tcp dport 80 dnat to 10.1.1.1:8080
	//   tcp dport 81 dnat to 10.1.1.1:1042
	//   fib daddr type local tcp dport 58080 dnat to 10.1.1.1:8080
	//  }
	//  chain svc_my-ns_my-svc_eps {
	//   numgen random mod 3 vmap {
	//     0: jump svc_my-ns_my-svc_ep_0a010001, 1: jump svc_my-ns_my-svc_ep_0a010002, 2: jump svc_my-ns_my-svc_ep_0a010101 }
	//  }
	//  chain svc_my-ns_my-svc_eps_metrics {
	//   numgen random mod 2 vmap {
	//     0: jump svc_my-ns_my-svc_ep_0a010002, 1: jump svc_my-ns_my-svc_ep_0a010101 }
	//  }
	//  chain svc_my-ns_my-svc_filter {
	//   tcp dport 82 reject
	//   fib daddr type local tcp dport 58081 reject
	//  }
	//  chain z_dispatch_svc_dnat {
	//   ip daddr vmap {
	//     10.0.0.1: jump svc_my-ns_my-svc_dnat }
	//  }
	//  chain z_dispatch_svc_filter {
	//   ip daddr vmap {
	//     10.0.0.1: jump svc_my-ns_my-svc_filter }
	//  }
	//  chain z_dnat_all {
	//   jump z_dispatch_svc_dnat
	//   fib daddr type local jump nodeports_dnat
	//  }
	//  chain z_filter_all {
	//   ct state invalid drop
	//   jump z_dispatch_svc_filter
	//   fib daddr type local jump nodeports_filter
	//  }
	//  chain z_hook_filter_forward {
	//   type filter hook forward priority 0;
	//   jump z_filter_all
	//  }
	//  chain z_hook_filter_output {
	//   type filter hook output priority 0;
	//   jump z_filter_all
	//  }
	//  chain z_hook_nat_output {
	//   type nat hook output priority 0;
	//   jump z_dnat_all
	//  }
	//  chain z_hook_nat_prerouting {
	//   type nat hook prerouting priority 0;
	//   jump z_dnat_all
	//  }
	//  chain zz_hook_nat_postrouting {
	//   type nat hook postrouting priority 0;
	//
	//   # masquerade non-cluster traffic to non-local endpoints
	//   ip saddr != { 10.1.0.0/16 } \
	//   ip daddr != { 10.1.0.1, 10.1.0.2 } \
	//   fib daddr type != local \
	//   masquerade
	//
	//   # masquerade hairpin traffic
	//   ip saddr . ip daddr { 10.1.0.1 . 10.1.0.1, 10.1.0.2 . 10.1.0.2 } masquerade
	//  }
	// }
}

func ExampleRenderServiceWithClientIPAffinity() {
	ctx, seps := testValues()

	seps.Service.SessionAffinity = &v1.Service_ClientIP{
		ClientIP: &v1.ClientIPAffinity{
			TimeoutSeconds: 30,
		},
	}

	ctx.addServiceEndpoints(seps)

	finalizeAndPrintTable(os.Stdout, ctx)

	// Output:
	// table ip k8s_svc {
	//  set svc_my-ns_my-svc_ep_0a010001_recent {
	//   type ipv4_addr; flags timeout;
	//  }
	//  set svc_my-ns_my-svc_ep_0a010002_recent {
	//   type ipv4_addr; flags timeout;
	//  }
	//  set svc_my-ns_my-svc_ep_0a010101_recent {
	//   type ipv4_addr; flags timeout;
	//  }
	//  chain nodeports_dnat {
	//   tcp dport 58080 jump svc_my-ns_my-svc_dnat
	//  }
	//  chain nodeports_filter {
	//   tcp dport 58081 jump svc_my-ns_my-svc_filter
	//  }
	//  chain svc_my-ns_my-svc_dnat {
	//   ip saddr @svc_my-ns_my-svc_ep_0a010001_recent jump svc_my-ns_my-svc_ep_0a010001
	//   ip saddr @svc_my-ns_my-svc_ep_0a010002_recent jump svc_my-ns_my-svc_ep_0a010002
	//   ip saddr @svc_my-ns_my-svc_ep_0a010101_recent jump svc_my-ns_my-svc_ep_0a010101
	//   tcp dport 80 jump svc_my-ns_my-svc_eps
	//   fib daddr type local tcp dport 58080 jump svc_my-ns_my-svc_eps
	//   tcp dport 81 jump svc_my-ns_my-svc_eps_metrics
	//  }
	//  chain svc_my-ns_my-svc_ep_0a010001 {
	//   update @svc_my-ns_my-svc_ep_0a010001_recent { ip saddr timeout 30s }
	//   tcp dport 80 dnat to 10.1.0.1:8080
	//   fib daddr type local tcp dport 58080 dnat to 10.1.0.1:8080
	//  }
	//  chain svc_my-ns_my-svc_ep_0a010002 {
	//   update @svc_my-ns_my-svc_ep_0a010002_recent { ip saddr timeout 30s }
	//   tcp dport 80 dnat to 10.1.0.2:8080
	//   tcp dport 81 dnat to 10.1.0.2:1011
	//   fib daddr type local tcp dport 58080 dnat to 10.1.0.2:8080
	//  }
	//  chain svc_my-ns_my-svc_ep_0a010101 {
	//   update @svc_my-ns_my-svc_ep_0a010101_recent { ip saddr timeout 30s }
	//   tcp dport 80 dnat to 10.1.1.1:8080
	//   tcp dport 81 dnat to 10.1.1.1:1042
	//   fib daddr type local tcp dport 58080 dnat to 10.1.1.1:8080
	//  }
	//  chain svc_my-ns_my-svc_eps {
	//   numgen random mod 3 vmap {
	//     0: jump svc_my-ns_my-svc_ep_0a010001, 1: jump svc_my-ns_my-svc_ep_0a010002, 2: jump svc_my-ns_my-svc_ep_0a010101 }
	//  }
	//  chain svc_my-ns_my-svc_eps_metrics {
	//   numgen random mod 2 vmap {
	//     0: jump svc_my-ns_my-svc_ep_0a010002, 1: jump svc_my-ns_my-svc_ep_0a010101 }
	//  }
	//  chain svc_my-ns_my-svc_filter {
	//   tcp dport 82 reject
	//   fib daddr type local tcp dport 58081 reject
	//  }
	//  chain z_dispatch_svc_dnat {
	//   ip daddr vmap {
	//     10.0.0.1: jump svc_my-ns_my-svc_dnat }
	//  }
	//  chain z_dispatch_svc_filter {
	//   ip daddr vmap {
	//     10.0.0.1: jump svc_my-ns_my-svc_filter }
	//  }
	//  chain z_dnat_all {
	//   jump z_dispatch_svc_dnat
	//   fib daddr type local jump nodeports_dnat
	//  }
	//  chain z_filter_all {
	//   ct state invalid drop
	//   jump z_dispatch_svc_filter
	//   fib daddr type local jump nodeports_filter
	//  }
	//  chain z_hook_filter_forward {
	//   type filter hook forward priority 0;
	//   jump z_filter_all
	//  }
	//  chain z_hook_filter_output {
	//   type filter hook output priority 0;
	//   jump z_filter_all
	//  }
	//  chain z_hook_nat_output {
	//   type nat hook output priority 0;
	//   jump z_dnat_all
	//  }
	//  chain z_hook_nat_prerouting {
	//   type nat hook prerouting priority 0;
	//   jump z_dnat_all
	//  }
	//  chain zz_hook_nat_postrouting {
	//   type nat hook postrouting priority 0;
	//
	//   # masquerade non-cluster traffic to non-local endpoints
	//   ip saddr != { 10.1.0.0/16 } \
	//   ip daddr != { 10.1.0.1, 10.1.0.2 } \
	//   fib daddr type != local \
	//   masquerade
	//
	//   # masquerade hairpin traffic
	//   ip saddr . ip daddr { 10.1.0.1 . 10.1.0.1, 10.1.0.2 . 10.1.0.2 } masquerade
	//  }
	// }
}

func finalizeAndPrintTable(out io.Writer, ctx *renderContext) {
	ctx.Finalize()
	defer ctx.table.Reset()

	printTable(out, ctx)
}

func printTable(out io.Writer, ctx *renderContext) {
	fmt.Fprintf(out, "table %s %s {\n", ctx.table.Family, ctx.table.Name)
	for _, ks := range ctx.table.KindStores() {
		items := ks.Store.List()
		if len(items) == 0 {
			continue
		}

		for _, item := range items {
			fmt.Fprintf(out, " %s %s {\n", ks.Kind, item.Key())
			io.Copy(out, item.Value())
			fmt.Fprintln(out, " }")
		}

	}
	fmt.Fprintln(out, "}")
}
