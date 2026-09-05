# zero-copy
A custom Go-based CNI plugin that implements a dual-path networking model: 1. Standard interface provisioning for cross-node pod communications. 2. An accelerated zero-copy path (utilizing AF_XDP socket buffers / shared memory) for intra-node pod communications.
