// +build ignore

#include <linux/bpf.h>
#include <linux/in.h>
#include <stdbool.h>
#include <errno.h>
#include "common.h"

#define SYS_REJECT 0
#define SYS_PROCEED 1

char __license[] SEC("license") = "Dual MIT/GPL";

const int DefaultMaxEntries = 65536;

struct V4_key {
  __be32 address;     /* Service virtual IPv4 address  4*/
  __be16 dport;       /* L4 port filter, if unset, all ports apply   */
  __u16 backend_slot; /* Backend iterator, 0 indicates the svc frontend  2*/
  //__u8 proto;		/* L4 protocol, currently not used (set to 0) 1 */
  //__u8 scope;		/* LB_LOOKUP_SCOPE_* for externalTrafficPolicy=Local 1*/
  //__u16 pad[2];
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
  __u8 proto;     /* L4 protocol, currently not used (set to 0) */
  __u8 flags;
};

struct bpf_map_def SEC("maps") v4_svc_map = {
    .type = BPF_MAP_TYPE_HASH,
    .key_size = sizeof(struct V4_key),
    .value_size = sizeof(struct lb4_service),
    .max_entries = DefaultMaxEntries,
};

struct bpf_map_def SEC("maps") v4_backend_map = {
    .type = BPF_MAP_TYPE_HASH,
    .key_size = sizeof(u32),
    .value_size = sizeof(struct lb4_backend),
    .max_entries = DefaultMaxEntries,
};

static __always_inline struct lb4_service *
lb4_lookup_service(struct V4_key *key) {
  struct lb4_service *svc;

  svc = bpf_map_lookup_elem(&v4_svc_map, key);
  if (svc) {
    // svc = bpf_map_lookup_elem(&v4_svc_map, key);
    // if (svc && svc->count)
    return svc;
  }

  // const char fmt_str[] = "No Service Found loading tmp service with key %x %x
  // %x \n";

  // bpf_trace_printk(fmt_str, sizeof(fmt_str), key->address, key->dport,
  // key->backend_slot);

  // struct lb4_service tmpsvc = { };
  // tmpsvc.count = 5;

  // bpf_map_update_elem(&v4_svc_map, key, &tmpsvc, BPF_ANY);

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

  const char fmt_str[] =
      "Hello, world, from BPF! I am in the program address is %x port is %x\n";

  bpf_trace_printk(fmt_str, sizeof(fmt_str), key.address, key.dport);

  // svc->count++;
  // bpf_map_update_elem(&v4_svc_map, &key, &svc, BPF_ANY);

  svc = lb4_lookup_service(&key);
  if (!svc) {
    return -ENXIO;
  }

  const char fmt_str2[] = "Hello, world, from BPF! I found a service %lu\n";

  bpf_trace_printk(fmt_str2, sizeof(fmt_str2), (unsigned long)svc->backend_id);

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

  // increment count to show hit
  // svc->count++;
  // bpf_map_update_elem(&v4_svc_map, &key, &svc, BPF_ANY);
  return 0;
}

SEC("cgroup/connect4")
int sock4_connect(struct bpf_sock_addr *ctx) {

  __sock4_fwd(ctx);
  return SYS_PROCEED;
}
