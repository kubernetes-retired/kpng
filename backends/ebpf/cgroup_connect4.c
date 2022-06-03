// +build ignore

#include <linux/bpf.h>
#include <linux/in.h>
#include <stdbool.h>
#include <errno.h>
#include "common.h"

#define SYS_REJECT 0
#define SYS_PROCEED 1
#define DEFAULT_MAX_EBPF_MAP_ENNTRIES 65536

char __license[] SEC("license") = "BSD-2-Clause";

struct V4_key {
  __be32 address;     /* Service virtual IPv4 address  4*/
  __be16 dport;       /* L4 port filter, if unset, all ports apply   */
  __u16 backend_slot; /* Backend iterator, 0 indicates the svc frontend  2*/
};

struct lb4_service {
  union {
    __u32 backend_id;       /* Backend ID in lb4_backends */
    __u32 affinity_timeout; /* In seconds, only for svc frontend */
    __u32 l7_lb_proxy_port; /* In host byte order, only when flags2 &&
                               SVC_FLAG_L7LOADBALANCER */
  };
  /* For the service frontend, count denotes number of service backend
   * slots (otherwise zero).
   */
  __u16 count;
  __u16 rev_nat_index; /* Reverse NAT ID in lb4_reverse_nat */
  __u8 flags;
  __u8 flags2;
  __u8 pad[2];
};

struct lb4_backend {
  __be32 address; /* Service endpoint IPv4 address */
  __be16 port;    /* L4 port filter */
  __u8 flags;
};

struct {
  __uint(type, BPF_MAP_TYPE_HASH); 
  __type(key, struct V4_key);
  __type(value, struct lb4_service); 
  __uint(max_entries, DEFAULT_MAX_EBPF_MAP_ENNTRIES);
} v4_svc_map SEC(".maps");

struct {
  __uint(type, BPF_MAP_TYPE_HASH); 
  __type(key, u32);
  __type(value, struct lb4_backend); 
  __uint(max_entries, DEFAULT_MAX_EBPF_MAP_ENNTRIES);
} v4_backend_map SEC(".maps");

static __always_inline struct lb4_service *
lb4_lookup_service(struct V4_key *key) {
  struct lb4_service *svc;

  svc = bpf_map_lookup_elem(&v4_svc_map, key);
  if (svc) {
    return svc;
  }

  return NULL;
}

/* Hack due to missing narrow ctx access. */
static __always_inline __be16 ctx_dst_port(const struct bpf_sock_addr *ctx) {
  volatile __u32 dport = ctx->user_port;

  return (__be16)dport;
}

static __always_inline __u64 sock_select_slot(struct bpf_sock_addr *ctx) {
  return ctx->protocol == IPPROTO_TCP ? bpf_get_prandom_u32() : 0;
}

static __always_inline struct lb4_backend *
__lb4_lookup_backend(__u32 backend_id) {
  return bpf_map_lookup_elem(&v4_backend_map, &backend_id);
}

static __always_inline struct lb4_service *
__lb4_lookup_backend_slot(struct V4_key *key) {
  return bpf_map_lookup_elem(&v4_svc_map, key);
}

/* Service translation logic for a local-redirect service can cause packets to
 * be looped back to a service node-local backend after translation. This can
 * happen when the node-local backend itself tries to connect to the service
 * frontend for which it acts as a backend. There are cases where this can break
 * traffic flow if the backend needs to forward the redirected traffic to the
 * actual service frontend. Hence, allow service translation for pod traffic
 * getting redirected to backend (across network namespaces), but skip service
 * translation for backend to itself or another service backend within the same
 * namespace. Currently only v4 and v4-in-v6, but no plain v6 is supported.
 *
 * For example, in EKS cluster, a local-redirect service exists with the AWS
 * metadata IP, port as the frontend <169.254.169.254, 80> and kiam proxy as a
 * backend Pod. When traffic destined to the frontend originates from the kiam
 * Pod in namespace ns1 (host ns when the kiam proxy Pod is deployed in
 * hostNetwork mode or regular Pod ns) and the Pod is selected as a backend, the
 * traffic would get looped back to the proxy Pod. Identify such cases by doing
 * a socket lookup for the backend <ip, port> in its namespace, ns1, and skip
 * service translation.
 */
static __always_inline bool
sock4_skip_xlate_if_same_netns(struct bpf_sock_addr *ctx,
                               const struct lb4_backend *backend) {
#ifdef BPF_HAVE_SOCKET_LOOKUP
  struct bpf_sock_tuple tuple = {
      .ipv4.daddr = backend->address,
      .ipv4.dport = backend->port,
  };
  struct bpf_sock *sk = NULL;

  switch (ctx->protocol) {
  case IPPROTO_TCP:
    sk = sk_lookup_tcp(ctx, &tuple, sizeof(tuple.ipv4), BPF_F_CURRENT_NETNS, 0);
    break;
  case IPPROTO_UDP:
    sk = sk_lookup_udp(ctx, &tuple, sizeof(tuple.ipv4), BPF_F_CURRENT_NETNS, 0);
    break;
  }

  if (sk) {
    sk_release(sk);
    return true;
  }
#endif /* BPF_HAVE_SOCKET_LOOKUP */
  return false;
}

static __always_inline void ctx_set_port(struct bpf_sock_addr *ctx,
                                         __be16 dport) {
  ctx->user_port = (__u32)dport;
}

static __always_inline int __sock4_fwd(struct bpf_sock_addr *ctx) {
  struct V4_key key = {
      .address = ctx->user_ip4,
      .dport = ctx_dst_port(ctx),
      .backend_slot = 0,
  };

  struct lb4_service *svc;
  struct lb4_service *backend_slot;
  struct lb4_backend *backend = NULL;

  __u32 backend_id = 0;

  svc = lb4_lookup_service(&key);
  if (!svc) {
    return -ENXIO;
  }

  // Logs are in /sys/kernel/debug/tracing/trace_pipe

  const char debug_str[] = "Entering the kpng ebpf backend, caught a\
  packet destined for my VIP, the address is: %x port is: %x and selected backend id is: %x\n";
  
  bpf_trace_printk(debug_str, sizeof(debug_str),  key.address, key.dport, svc->backend_id);

  if (backend_id == 0) {
    key.backend_slot = (sock_select_slot(ctx) % svc->count) + 1;
    backend_slot = __lb4_lookup_backend_slot(&key);
    if (!backend_slot) {
      return -ENOENT;
    }

    backend_id = backend_slot->backend_id;
    backend = __lb4_lookup_backend(backend_id);
  }

  if (!backend) {
    return -ENOENT;
  }

  if (sock4_skip_xlate_if_same_netns(ctx, backend)) {
    return -ENXIO;
  }

  ctx->user_ip4 = backend->address;
  ctx_set_port(ctx, backend->port);

  return 0;
}

SEC("cgroup/connect4")
int sock4_connect(struct bpf_sock_addr *ctx) {

  __sock4_fwd(ctx);
  return SYS_PROCEED;
}
