# Zero-Copy CNI Plugin — Implementation Plan

## Context

The project at `/home/ahsan/Desktop/zero-copy` is a Kubernetes CNI plugin that implements intra-node zero-copy pod-to-pod networking using eBPF/XDP. All Go source files are empty stubs, all YAML manifests are empty, and the Makefile/Dockerfile are empty. The only complete file is `ebpf/fast-path.c`. This plan covers implementing everything needed to make the project functional end-to-end.

---

## Prerequisites

Before implementing any Go code, compile the eBPF C program:
- Install deps via `script.sh`: `clang`, `llvm`, `libbpf-dev`, `linux-headers-$(uname -r)`
- Compile `ebpf/fast-path.c` → `ebpf/fast-path.o` using `clang -target bpf -c fast-path.c -o fast-path.o`
- The Go eBPF loader will load this `.o` file at runtime

---

## Task 1: Implement `pkg/network/netns.go` — Network Namespace Operations

**What to implement:**
- `MoveToNetNS()` — Enter a network namespace given its path
  - Use `unix.Setns(fd, unix.CLONE_NEWNET)` system call
  - Need to `open()` the netns path first to get a file descriptor
- `ConfigureAddress()` — Configure loopback interface inside a netns
  - Use `ioctl(fd, SIOCGIFFLAGS)` / `SIOCSIFFLAGS` to bring `lo` UP
  - Use netlink (`AF_NETLINK` socket) for IP address assignment if needed

**Key system calls:** `open`, `setns`, `ioctl`

---

## Task 2: Implement `pkg/network/veth.go` — Virtual Ethernet Pair Operations

**What to implement:**
- `CreateVeth()` — Create a veth pair and distribute ends across namespaces
  - Use `socket(AF_INET, SOCK_DGRAM, 0)` + `ioctl(TUNSETIFF)` to create pair
  - Use `ioctl(SIOCGIFINDEX)` to get ifindex of each end
  - Use `setns()` to move one end into pod netns, other stays on host
  - Bring both interfaces UP using `ioctl(SIOCSIFFLAGS)`
  - Assign IP address to pod-side veth
- `DeleteVeth()` — Delete a veth pair
  - Use `ioctl(SIOCADDMAP)` or netlink `RTM_DELLINK` to delete interface
  - Operate from host namespace

**Key system calls:** `socket`, `ioctl` (TUNSETIFF, SIOCGIFINDEX, SIOCSIFFLAGS), `setns`

---

## Task 3: Implement `pkg/network/route.go` — Routing Table Operations

**What to implement:**
- `AddRoute()` — Add a route to the pod subnet via host veth
  - Create `AF_NETLINK` socket
  - Send `RTM_NEWROUTE` netlink message with `rtm_dst_len`, `rtm_goal`, `rtm_oif`
  - Use `rtnl_route_add()` from libnl OR manual netlink message construction
- `DeleteRoute()` — Remove the route
  - Send `RTM_DELROUTE` netlink message

**Key system calls:** `socket(AF_NETLINK)`, `sendmsg`, `recvmsg`

**Note:** Consider using `github.com/vishvananda/netlink` library to simplify netlink operations — check if it's in go.mod or needs adding.

---

## Task 4: Implement `pkg/ebpf/maps.go` — BPF Map Management

**What to implement:**
- Function to create a BPF hash map (`pod_interface_map`)
  - Use `unix.Bpf(unix.BPF_MAP_CREATE, &attr)` system call
  - Type: `BPF_MAP_TYPE_HASH`, KeySize: 4 (uint32 IP), ValueSize: 4 (uint32 ifindex), MaxEntries: 65535
- Function to update an entry (pod IP → veth ifindex)
  - Use `unix.Bpf(unix.BPF_MAP_UPDATE_ELEM, ...)`
- Function to delete an entry (on pod deletion)
  - Use `unix.Bpf(unix.BPF_MAP_DELETE_ELEM, ...)`
- Function to look up an entry
  - Use `unix.Bpf(unix.BPF_MAP_LOOKUP_ELEM, ...)`

**Key system calls:** `bpf(BPF_MAP_*)`

---

## Task 5: Implement `pkg/ebpf/loader.go` — eBPF Program Loading

**What to implement:**
- Function to read compiled `.o` ELF file from disk
- Function to load the XDP program into kernel
  - Use `unix.Bpf(unix.BPF_PROG_LOAD, &attr)` system call
  - Attr.Type = `BPF_PROG_TYPE_XDP`
  - Attr.Insn = bytecode from ELF, Attr.InsnCnt = instruction count
  - Attr.License = "GPL"
- Return the program file descriptor

**Key system calls:** `bpf(BPF_PROG_LOAD)`, `open`, `read`, `close`

**Dependency needed:** Need to parse ELF format — either manually or using `github.com/cilium/ebpf` library (recommended, handles ELF parsing automatically).

---

## Task 6: Implement `pkg/ebpf/programs.go` — XDP Program Attachment

**What to implement:**
- Function to attach XDP program to a network interface
  - Open the host-side veth with `socket()`
  - Use `setsockopt(fd, SOL_SOCKET, SO_ATTACH_XDP, &prog_fd)` system call
- Function to detach XDP program from interface
  - Use `setsockopt(fd, SOL_SOCKET, SO_DETACH_XDP, 0)`

**Key system calls:** `socket`, `setsockopt`

---

## Task 7: Implement `cmd/cni/main.go` — CNI Entry Point

**What to implement:**
- Parse CNI JSON input from stdin (standard CNI spec: contains command, container ID, netns, ifName, args, config)
- Based on `cmd`:
  - `"ADD"` → call `pkg/cni/add.go` AddPod()
  - `"DEL"` → call `pkg/cni/del.go` DeletePod()
- Write CNI result JSON to stdout on success (with IP, gateway, routes)
- Write error JSON to stderr on failure

**Reference:** CNI v1.0 spec at https://github.com/container-interface/cni/blob/master/spec.md

---

## Task 8: Implement `pkg/cni/add.go` — Pod ADD Logic

**What to implement:** `AddPod(args *cni.Args) error`

Full orchestration in correct order:
1. Parse CNI config (pod IP, subnet, gateway, netns path)
2. `netns.MoveToNetNS()` — enter pod's namespace
3. `netns.ConfigureAddress()` — set up loopback
4. `network.CreateVeth()` — create veth pair (veth-podX ↔ veth-hostX)
5. `netns.ConfigureAddress()` — assign pod IP to veth-podX inside pod netns
6. `network.AddRoute()` — add subnet route via veth-hostX on host
7. `ebpf.LoadProgram()` — load fast-path.o into kernel
8. `ebpf.CreateMap()` — create pod_interface_map
9. `ebpf.UpdateMap()` — add entries for all peer pods on same node (for each co-located pod, add its IP → its host-veth ifindex)
10. `ebpf.AttachXDP()` — attach XDP program to host-side veths

---

## Task 9: Implement `pkg/cni/del.go` — Pod DEL Logic

**What to implement:** `DeletePod(args *cni.Args) error`

Reverse order of ADD:
1. `ebpf.DetachXDP()` — detach XDP from host veth
2. `ebpf.DeleteMap()` — remove pod IP entry from pod_interface_map
3. `network.DeleteRoute()` — remove route entry
4. `network.DeleteVeth()` — delete veth pair
5. Remove pod's network namespace

---

## Task 10: Write `Makefile`

**What to implement:**
```makefile
# Targets:
all: build

build:
	go build -o bin/cni ./cmd/cni

build-ebpf:
	clang -target bpf -c ebpf/fast-path.c -o ebpf/fast-path.o

clean:
	rm -rf bin/ ebpf/*.o

install:
	cp bin/cni /opt/cni/bin/zero-copy
	cp deploy/ /etc/cni/net.d/

test:
	go test ./...
```

---

## Task 11: Write `Dockerfile`

**What to implement:**
- Base image: `golang:1.26-alpine` + `ubuntu` (for libbpf)
- Install build deps: clang, llvm, libbpf-dev, linux-headers
- Copy source, compile eBPF, build Go binary
- Entrypoint: run the CNI binary

---

## Task 12: Write `deploy/cni-config.yaml`

**What to implement:**
CNI network config (placed at `/etc/cni/net.d/`):
```yaml
apiVersion: v1
kind: NetworkConfig
name: zero-copy
cniVersion: "1.0.0"
plugins:
  - type: zero-copy
    subnet: "10.1.0.0/24"
    gateway: "10.1.0.1"
```

---

## Task 13: Write `deploy/rbac.yaml`

**What to implement:**
- ServiceAccount for the DaemonSet
- ClusterRole with permissions: `hostNetwork`, `hostPID`, capabilities `NET_ADMIN`, `SYS_ADMIN`, access to `/sys/fs/bpf`, `/proc/*/ns/net`
- ClusterRoleBinding

---

## Task 14: Write `deploy/daemonset.yaml`

**What to implement:**
- DaemonSet running on all nodes
- Host networking (`hostNetwork: true`)
- Host PID (`hostPID: true`)
- Privileged container
- Mount `/sys/fs/bpf`, `/lib/modules`, `/run/netns`
- Runs the CNI binary as the main process

---

## Task 15: Add Dependencies to `go.mod`

**Recommended libraries:**
- `github.com/vishvananda/netlink` — netlink socket operations (routes, interfaces)
- `github.com/cilium/ebpf` — eBPF program loading, ELF parsing, map management (simplifies Tasks 4-6 dramatically)
- `github.com/containernetworking/cni` — CNI spec types and parsing (Task 7)

---

## Implementation Order (Dependencies)

```
Task 1 (netns)         ← no dependencies
Task 2 (veth)          ← depends on Task 1 (needs setns)
Task 3 (route)         ← no direct dependency (but used by Task 8)
Task 4 (maps)          ← no direct dependency
Task 5 (loader)        ← no direct dependency
Task 6 (programs)      ← depends on Task 4 (needs map fd), Task 5 (needs prog fd)
Task 15 (deps)         ← do early, affects all ebpf/network tasks
Task 7 (main.go)       ← depends on Tasks 8, 9 (call them)
Task 8 (add.go)        ← depends on Tasks 1, 2, 3, 4, 5, 6
Task 9 (del.go)        ← depends on Tasks 1, 2, 4, 6
Task 10 (Makefile)     ← no dependency
Task 11 (Dockerfile)   ← depends on Task 10
Task 12 (cni-config)   ← no dependency
Task 13 (rbac)         ← no dependency
Task 14 (daemonset)    ← no dependency
```

---

## Validation Plan

1. **Unit tests:** Write Go tests for each package using `go test ./pkg/...` (mock system calls)
2. **eBPF verification:** Use `bpftool prog show` and `bpftool map dump` to verify program loaded and map populated
3. **Integration test:** Run in a local Kubernetes cluster (kind/minikube):
   - Deploy the DaemonSet
   - Create two pods on the same node
   - Run `iperf3` between them → verify throughput improvement
   - Check `bpftool` output confirms XDP program attached
4. **CNI validation:** Run `sudo CNI_COMMAND=ADD CNI_NETNS=<path> /opt/cni/bin/zero-copy` manually on a test namespace
