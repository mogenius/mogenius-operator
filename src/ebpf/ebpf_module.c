//go:build ignore
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 2);
} xdp_counter_state SEC(".maps");

// count_packets atomically increases a packet counter and byte counter on every invocation.
SEC("xdp")
int update_xdp(struct xdp_md *ctx) {
    __u32 rx_pkt_key  = 0;
    __u32 rx_byte_key = 1;

    // Count received packets
    __u64 *rx_pkt_count = bpf_map_lookup_elem(&xdp_counter_state, &rx_pkt_key);
    if (rx_pkt_count) {
        __sync_fetch_and_add(rx_pkt_count, 1);
    }

    // Count received bytes
    __u64 *rx_byte_count = bpf_map_lookup_elem(&xdp_counter_state, &rx_byte_key);
    if (rx_byte_count) {
        __sync_fetch_and_add(rx_byte_count, (__u64)ctx->data_end - (__u64)ctx->data);
    }

    return XDP_PASS;
}

__u64 ingress_pkt_count = 0;
__u64 ingress_bytes = 0;
__u64 egress_pkt_count = 0;
__u64 egress_bytes = 0;

SEC("tc")
int update_tc_ingress(struct __sk_buff *skb) {
    __sync_fetch_and_add(&ingress_pkt_count, 1);
    __sync_fetch_and_add(&ingress_bytes, (__u64)skb->data_end - (__u64)skb->data);
	return TCX_PASS;
}

SEC("tc")
int update_tc_egress(struct __sk_buff *skb) {
    __sync_fetch_and_add(&egress_pkt_count, 1);
    __sync_fetch_and_add(&egress_bytes, (__u64)skb->data_end - (__u64)skb->data);
	return TCX_PASS;
}
