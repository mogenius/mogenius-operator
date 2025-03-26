//go:build ignore
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 2); // 0: pkt_count, 1: byte_count
} counters SEC(".maps");

// count_packets atomically increases a packet counter and byte counter on every invocation.
SEC("xdp")
int count_packets(struct xdp_md *ctx) {
    __u32 pkt_key  = 0;
    __u32 byte_key = 1;

    // Count packets
    __u64 *pkt_count = bpf_map_lookup_elem(&counters, &pkt_key);
    if (pkt_count) {
        __sync_fetch_and_add(pkt_count, 1);
    }

    // Count bytes
    __u64 *byte_count = bpf_map_lookup_elem(&counters, &byte_key);
    if (byte_count) {
        __sync_fetch_and_add(byte_count, (__u64)ctx->data_end - (__u64)ctx->data);
    }

    return XDP_PASS;
}
