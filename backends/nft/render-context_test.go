package nft

import (
	"fmt"
	"io"
	"net"
	"os"

	v1 "sigs.k8s.io/kpng/api/localnetv1"
	"sigs.k8s.io/kpng/client/localsink/fullstate"
)

func ExampleRenderContext() {
	ctx := newRenderContext(table4, []string{"10.1.0.0/16"}, net.CIDRMask(24, 32))

	svc := &v1.Service{
		Namespace: "my-ns",
		Name:      "my-svc",
		Type:      "ClusterIP",
		IPs: &v1.ServiceIPs{
			ClusterIPs: v1.NewIPSet("10.0.0.1"),
		},
		Ports: []*v1.PortMapping{
			{Name: "http", Protocol: v1.Protocol_TCP, Port: 80, TargetPort: 8080, NodePort: 58080},
		},
	}

	seps := &fullstate.ServiceEndpoints{
		Service: svc,
		Endpoints: []*v1.Endpoint{
			{IPs: v1.NewIPSet("10.1.0.1"), Local: true},
			{IPs: v1.NewIPSet("10.1.1.1"), Local: false},
		},
	}

	ctx.addServiceEndpoints(seps)

	fmt.Println("-- basic svc --")
	printTable(os.Stdout, ctx)

	fmt.Println()
	fmt.Println("-- svc with client IP affinity --")

	svc.SessionAffinity = &v1.Service_ClientIP{
		ClientIP: &v1.ClientIPAffinity{
			TimeoutSeconds: 30,
		},
	}

	ctx.addServiceEndpoints(seps)

	printTable(os.Stdout, ctx)

	// Output:
	// -- basic svc --
	// table ip k8s_svc {
	//  map endpoints_0008 {
	//   typeof numgen random mod 1 : ip daddr
	//   elements = {\
	//     0 : 10.1.0.1, 1 : 10.1.1.1}
	//  }
	//  chain dnat_external {
	//  }
	//  chain dnat_net_0a000000 {
	//   ip daddr vmap { 10.0.0.1: jump dnat_svc_my-ns_my-svc}
	//  }
	//  chain dnat_svc_my-ns_my-svc {
	//   tcp dport 80 counter dnat to numgen random mod 2 offset 0 map @endpoints_0008:8080
	//   fib daddr type local tcp dport 58080 counter dnat to numgen random mod 2 offset 0 map @endpoints_0008:8080
	//  }
	//  chain filter_external {
	//  }
	//  chain hook_filter_forward {
	//   type filter hook forward priority 0;
	//   jump z_filter_all
	//  }
	//  chain hook_filter_output {
	//   type filter hook output priority 0;
	//   jump z_filter_all
	//  }
	//  chain hook_nat_output {
	//   type nat hook output priority 0;
	//   jump z_dnat_all
	//  }
	//  chain hook_nat_postrouting {
	//   type nat hook postrouting priority 0;
	//
	//   # masquerade non-cluster traffic to non-local endpoints
	//   ip saddr != { 10.1.0.0/16 } \
	//   ip daddr != { 10.1.0.1 } \
	//   masquerade
	//
	//   # masquerade hairpin traffic
	//   ip saddr . ip daddr { 10.1.0.1 . 10.1.0.1 } masquerade
	//  }
	//  chain hook_nat_prerouting {
	//   type nat hook prerouting priority 0;
	//   jump z_dnat_all
	//  }
	//  chain nodeports {
	//   tcp dport 58080 jump dnat_svc_my-ns_my-svc
	//  }
	//  chain z_dnat_all {
	//   ip daddr vmap { 10.0.0.0/24: jump dnat_net_0a000000}
	//   fib daddr type local jump nodeports
	//  }
	//  chain z_filter_all {
	//   ct state invalid drop
	//  }
	// }
	//
	// -- svc with client IP affinity --
	// table ip k8s_svc {
	//  set dnat_svc_my-ns_my-svc_epset_0a010001 {
	//   typeof ip daddr; flags timeout;
	//  }
	//  set dnat_svc_my-ns_my-svc_epset_0a010101 {
	//   typeof ip daddr; flags timeout;
	//  }
	//  chain dnat_external {
	//  }
	//  chain dnat_net_0a000000 {
	//   ip daddr vmap { 10.0.0.1: jump dnat_svc_my-ns_my-svc}
	//  }
	//  chain dnat_svc_my-ns_my-svc {
	//   ip saddr @dnat_svc_my-ns_my-svc_epset_0a010001 jump dnat_svc_my-ns_my-svc_ep_0a010001
	//   ip saddr @dnat_svc_my-ns_my-svc_epset_0a010101 jump dnat_svc_my-ns_my-svc_ep_0a010101
	//   numgen random mod 2 vmap { 0: jump dnat_svc_my-ns_my-svc_ep_0a010001, 1: jump dnat_svc_my-ns_my-svc_ep_0a010101}
	//  }
	//  chain dnat_svc_my-ns_my-svc_ep_0a010001 {
	//   update @dnat_svc_my-ns_my-svc_epset_0a010001 { ip saddr timeout 30s }
	//   tcp dport 80 dnat to 10.1.0.1:8080
	//   fib daddr type local tcp dport 58080 dnat to 10.1.0.1:8080
	//  }
	//  chain dnat_svc_my-ns_my-svc_ep_0a010101 {
	//   update @dnat_svc_my-ns_my-svc_epset_0a010101 { ip saddr timeout 30s }
	//   tcp dport 80 dnat to 10.1.1.1:8080
	//   fib daddr type local tcp dport 58080 dnat to 10.1.1.1:8080
	//  }
	//  chain filter_external {
	//  }
	//  chain hook_filter_forward {
	//   type filter hook forward priority 0;
	//   jump z_filter_all
	//  }
	//  chain hook_filter_output {
	//   type filter hook output priority 0;
	//   jump z_filter_all
	//  }
	//  chain hook_nat_output {
	//   type nat hook output priority 0;
	//   jump z_dnat_all
	//  }
	//  chain hook_nat_postrouting {
	//   type nat hook postrouting priority 0;
	//
	//   # masquerade non-cluster traffic to non-local endpoints
	//   ip saddr != { 10.1.0.0/16 } \
	//   ip daddr != { 10.1.0.1 } \
	//   masquerade
	//
	//   # masquerade hairpin traffic
	//   ip saddr . ip daddr { 10.1.0.1 . 10.1.0.1 } masquerade
	//  }
	//  chain hook_nat_prerouting {
	//   type nat hook prerouting priority 0;
	//   jump z_dnat_all
	//  }
	//  chain nodeports {
	//   tcp dport 58080 jump dnat_svc_my-ns_my-svc
	//  }
	//  chain z_dnat_all {
	//   fib daddr type local jump nodeports
	//  }
	//  chain z_filter_all {
	//   ct state invalid drop
	//  }
	// }

}

func printTable(out io.Writer, ctx *renderContext) {
	ctx.Finalize()
	defer ctx.table.Reset()

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
