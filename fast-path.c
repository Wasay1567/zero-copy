// fast_path.c
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include <linux/if_ether.h>
#include <linux/ip.h>

// 1. Define the hash map shared between Go and C
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32); // Key: Local Pod IP (IPv4 as integer)
    __type(value, __u32); // Value: interface index (ifindex) for a Linux virtual ethernet (veth) 
    __uint(max_entries, 65535);
} pod_interface_map SEC(".maps");

SEC("xdp")
int xdp_pass(struct xdp_md* ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data     = (void *)(long)ctx->data;

    struct ethhdr *eth = data;  
    // check if data in ethernet header is of strictly 14 bytes and not corrupted or malformed.
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;

    // 2. Check if it's IPv4
    // ETH_P_IP is hex code of representing standard IPv4 protocol
    if (eth->h_proto != bpf_htons(ETH_P_IP)) 
        return XDP_PASS;

    // 3. Verify IP header
    // eth + 1 is where the ethernet header ends and ip header begins
    struct iphdr *ip = (void *)(eth + 1);
    // adding 1 to an iphdr pointer jumps forward by the entire size of an IP header (usually 20 bytes)
    if ((void *)(ip + 1) > data_end)
        return XDP_PASS;

    if (ip->version != 4)
        return XDP_PASS;

    if (ip->ihl < 5)
        return XDP_PASS;

    void *ip_end = (void *)ip + (ip->ihl * 4);
    if (ip_end > data_end)
        return XDP_PASS;

    // 4. Get destination IP address (network byte order)
    __u32 dst_ip = ip->daddr;
    __u32 *ifindex = bpf_map_lookup_elem(&pod_interface_map, &dst_ip);

    if (!ifindex)
        return XDP_PASS;

    return bpf_redirect_peer(*ifindex, 0);
    
    return XDP_PASS;
}



// License metadata required by the Linux kernel
char _license[] SEC("license") = "GPL";