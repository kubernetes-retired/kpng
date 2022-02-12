package nft

import "os"

func ExampleSvcVmap() {
	ctx, seps := testValues()
	ctx.addSvcVmap("my-vmap", seps.Service, ctx.epIPs(seps.Endpoints))
	printTable(os.Stdout, ctx)

	// Output:
	// table ip k8s_svc {
	//  chain my-vmap {
	//   numgen random mod 3 vmap {
	//     0: jump svc_my-ns_my-svc_ep_0a010001, 1: jump svc_my-ns_my-svc_ep_0a010002, 2: jump svc_my-ns_my-svc_ep_0a010101 }
	//  }
	// }
}

func ExampleSvcChain() {
	ctx, seps := testValues()
	ctx.addSvcChain(seps.Service, ctx.epIPs(seps.Endpoints))
	printTable(os.Stdout, ctx)

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
	// }

}
