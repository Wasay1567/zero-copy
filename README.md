#Intra-Node Zero-Copy Acceleration CNI Plugin for Kubernetes

Problem Statement:
Standard CNI plugins (e.g., Flannel, Calico) process all pod-to-pod traffic through the host Linux kernel network stack (sk_buff allocations, iptables/routing lookups, and context switches). For co-located pods (e.g., microservices running on the same node), this introduces unnecessary packet copying and latency.

Proposed Solution:
A custom Go-based CNI plugin that implements a dual-path networking model:
1. Standard interface provisioning for cross-node pod communications.
2. An accelerated zero-copy path (utilizing AF_XDP socket buffers / shared memory) for intra-node pod communications.

Key Objectives:
- Implement a CNI binary conforming to the CNI v1.0 specification (handling ADD/DEL commands).
- Establish shared memory / AF_XDP socket pairs for same-node pod communication.
- Benchmark and compare throughput (Gbps), latency (μs), and CPU overhead against standard CNI plugins using iperf3.
