//go:build ignore
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

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
