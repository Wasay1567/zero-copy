#!/usr/bin/env bash
#
# test-xdp-intra-node-redirect.sh
#
# End-to-end test for the XDP fast-path pod-to-pod redirect (fast-path.c).
#
# What it proves:
#   Two isolated network namespaces (ns-a, ns-b) can exchange traffic using
#   ONLY the XDP fast path (a BPF map lookup on destination IP followed by
#   bpf_redirect()) — with no bridge, no route, and no other Linux
#   forwarding mechanism connecting them.
#
# Workflow: setup -> test -> debug (on failure) -> cleanup (always).
#
# Usage:
#   sudo ./test-xdp-intra-node-redirect.sh
#
# Requirements: root privileges. Required tools (clang, bpftool, iproute2,
# ethtool) are checked for automatically and, on apt-based systems, missing
# ones are installed before the test runs.

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
NS_A="ns-a"
NS_B="ns-b"

VETH_A_HOST="veth-a-host"   # root-namespace end of ns-a's veth pair
VETH_A_PEER="veth-a"        # ns-a-namespace end of the same pair
VETH_B_HOST="veth-b-host"   # root-namespace end of ns-b's veth pair
VETH_B_PEER="veth-b"        # ns-b-namespace end of the same pair

IP_A="10.10.1.2"
IP_B="10.10.1.3"
SUBNET="10.10.1.0/24"

BPF_SRC="fast-path.c"
BPF_OBJ="fast_path.o"
PIN_DIR="/sys/fs/bpf/pod-to-pod-xdp"
PIN_MAPS_DIR="${PIN_DIR}/maps"
XDP_PROG_PIN="${PIN_DIR}/xdp_pass"

PING_COUNT=4

# Tracks whether setup got far enough that cleanup has real work to do.
SETUP_STARTED=0

# ---------------------------------------------------------------------------
# Logging helpers
# ---------------------------------------------------------------------------
log()   { echo -e "\n[*] $*"; }
ok()    { echo "[OK] $*"; }
fail()  { echo "[FAIL] $*" >&2; }

# ---------------------------------------------------------------------------
# Cleanup: always runs on exit (success, failure, or Ctrl+C), via trap.
# Safe to run even if setup only partially completed.
# ---------------------------------------------------------------------------
cleanup() {
    local exit_code=$?

    if [[ "$SETUP_STARTED" -eq 0 ]]; then
        exit "$exit_code"
    fi

    log "Cleaning up test environment..."

    # Detach XDP programs (ignore errors if already detached/missing)
    sudo ip link set dev "$VETH_A_HOST" xdp off 2>/dev/null || true
    sudo ip link set dev "$VETH_B_HOST" xdp off 2>/dev/null || true

    # Remove pinned BPF program + maps
    sudo rm -rf "$PIN_DIR"

    # Delete veth pairs (deleting the host end also removes the peer)
    sudo ip link del "$VETH_A_HOST" 2>/dev/null || true
    sudo ip link del "$VETH_B_HOST" 2>/dev/null || true

    # Delete network namespaces
    sudo ip netns del "$NS_A" 2>/dev/null || true
    sudo ip netns del "$NS_B" 2>/dev/null || true

    ok "Cleanup complete."
    exit "$exit_code"
}
trap cleanup EXIT INT TERM

# ---------------------------------------------------------------------------
# Step 0: Check prerequisites and install anything missing (apt-based
# systems only). Each tool maps to the apt package that provides it.
# ---------------------------------------------------------------------------
declare -A REQUIRED_TOOLS=(
    [clang]="clang libbpf-dev"
    [bpftool]="linux-tools-common linux-tools-$(uname -r) linux-tools-generic"
    [ip]="iproute2"
    [ethtool]="ethtool"
    [bridge]="iproute2"
    [ping]="iputils-ping"
)

check_prerequisites() {
    log "Checking prerequisites..."

    if [[ "${EUID}" -ne 0 ]]; then
        fail "This script must be run as root (or via sudo). Re-run as: sudo $0"
        exit 1
    fi

    local missing_tools=()
    local missing_packages=()

    for tool in "${!REQUIRED_TOOLS[@]}"; do
        if command -v "$tool" >/dev/null 2>&1; then
            echo "    [found] ${tool}"
        else
            echo "    [missing] ${tool}"
            missing_tools+=("$tool")
            # shellcheck disable=SC2206
            missing_packages+=(${REQUIRED_TOOLS[$tool]})
        fi
    done

    if [[ "${#missing_tools[@]}" -eq 0 ]]; then
        ok "All prerequisites are present."
        return 0
    fi

    if ! command -v apt-get >/dev/null 2>&1; then
        fail "Missing tools (${missing_tools[*]}) and apt-get is not available to auto-install them."
        fail "Please install these manually for your distro and re-run."
        exit 1
    fi

    log "Attempting to install missing packages via apt-get: ${missing_packages[*]}"
    apt-get update -y
    # Install packages individually so one unavailable candidate (e.g. a
    # kernel-specific linux-tools-<version> package) doesn't abort the rest.
    for pkg in "${missing_packages[@]}"; do
        apt-get install -y "$pkg" || echo "    [warn] could not install ${pkg}, continuing..."
    done

    # Re-check after attempting installation.
    local still_missing=()
    for tool in "${missing_tools[@]}"; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            still_missing+=("$tool")
        fi
    done

    if [[ "${#still_missing[@]}" -gt 0 ]]; then
        fail "Still missing after install attempt: ${still_missing[*]}"
        fail "Please install these manually (bpftool commonly needs a matching linux-tools-\$(uname -r) package) and re-run."
        exit 1
    fi

    ok "All prerequisites installed."
}

# ---------------------------------------------------------------------------
# Step 1: Create namespaces, veth pairs, addresses, and enable GRO
# ---------------------------------------------------------------------------
setup_namespaces() {
    log "Creating network namespaces and veth pairs..."
    SETUP_STARTED=1

    sudo ip netns add "$NS_A"
    sudo ip netns add "$NS_B"

    # ns-a side
    sudo ip link add "$VETH_A_HOST" type veth peer name "$VETH_A_PEER" netns "$NS_A"
    sudo ip netns exec "$NS_A" ip link set lo up
    sudo ip netns exec "$NS_A" ip link set "$VETH_A_PEER" up
    sudo ip netns exec "$NS_A" ip addr add "${IP_A}/24" dev "$VETH_A_PEER"
    sudo ip link set "$VETH_A_HOST" up

    # ns-b side
    sudo ip link add "$VETH_B_HOST" type veth peer name "$VETH_B_PEER" netns "$NS_B"
    sudo ip netns exec "$NS_B" ip link set lo up
    sudo ip netns exec "$NS_B" ip link set "$VETH_B_PEER" up
    sudo ip netns exec "$NS_B" ip addr add "${IP_B}/24" dev "$VETH_B_PEER"
    sudo ip link set "$VETH_B_HOST" up

    # GRO required so NAPI mode accepts frames delivered via bpf_redirect()'s
    # ndo_xdp_xmit; without it, redirected frames are silently dropped.
    sudo ip netns exec "$NS_A" ethtool -K "$VETH_A_PEER" gro on
    sudo ip netns exec "$NS_B" ethtool -K "$VETH_B_PEER" gro on

    ok "Namespaces and veth pairs are up."
}

# ---------------------------------------------------------------------------
# Step 2: Compile the eBPF program
# ---------------------------------------------------------------------------
compile_bpf() {
    log "Compiling ${BPF_SRC}..."

    clang -O2 -g -target bpf -D__TARGET_ARCH_x86 \
        -I/usr/include/x86_64-linux-gnu \
        -c "$BPF_SRC" -o "$BPF_OBJ"

    ok "Compiled ${BPF_OBJ}."
}

# ---------------------------------------------------------------------------
# Step 3: Load once, pin program + map, attach to both veth-host interfaces
# ---------------------------------------------------------------------------
load_and_attach_bpf() {
    log "Loading and pinning BPF program (single instance, shared map)..."

    sudo mkdir -p "$PIN_MAPS_DIR"
    sudo bpftool prog loadall "$BPF_OBJ" "$PIN_DIR" pinmaps "$PIN_MAPS_DIR"

    # Attaching the SAME pinned program to both interfaces ensures they
    # share one pod_interface_map instance (loading separately per
    # interface would create two disconnected maps and break bidirectional
    # lookups).
    sudo ip link set dev "$VETH_A_HOST" xdp pinned "$XDP_PROG_PIN"
    sudo ip link set dev "$VETH_B_HOST" xdp pinned "$XDP_PROG_PIN"

    ok "XDP program attached to ${VETH_A_HOST} and ${VETH_B_HOST}."
    sudo bpftool net show
}

# ---------------------------------------------------------------------------
# Helper: convert an integer ifindex into the 4-byte little-endian hex
# string bpftool's "value hex ..." syntax expects (e.g. "02 00 00 00").
# Pure bash — no python3 dependency.
# ---------------------------------------------------------------------------
ifindex_to_le_hex() {
    local ifindex=$1
    printf '%02x %02x %02x %02x' \
        "$(( ifindex & 0xff ))" \
        "$(( (ifindex >> 8) & 0xff ))" \
        "$(( (ifindex >> 16) & 0xff ))" \
        "$(( (ifindex >> 24) & 0xff ))"
}

# ---------------------------------------------------------------------------
# Step 4: Populate the shared BPF map with ifindex redirects
# ---------------------------------------------------------------------------
populate_map() {
    log "Populating pod_interface_map with destination -> ifindex entries..."

    local ifindex_a ifindex_b map_pin val_a_hex val_b_hex

    ifindex_a=$(cat "/sys/class/net/${VETH_A_HOST}/ifindex")
    ifindex_b=$(cat "/sys/class/net/${VETH_B_HOST}/ifindex")
    echo "    ${VETH_A_HOST} ifindex = ${ifindex_a}"
    echo "    ${VETH_B_HOST} ifindex = ${ifindex_b}"

    # Map name is truncated to 15 chars by the kernel (pod_interface_m);
    # find the pinned map file rather than hardcoding the truncated name.
    map_pin="${PIN_MAPS_DIR}/$(sudo ls "$PIN_MAPS_DIR")"
    echo "    Using map: ${map_pin}"

    val_a_hex=$(ifindex_to_le_hex "$ifindex_a")
    val_b_hex=$(ifindex_to_le_hex "$ifindex_b")

    # dst 10.10.1.3 -> go OUT veth-b-host (so it lands in ns-b)
    sudo bpftool map update pinned "$map_pin" key hex 0a 0a 01 03 value hex $val_b_hex
    # dst 10.10.1.2 -> go OUT veth-a-host (so it lands in ns-a)
    sudo bpftool map update pinned "$map_pin" key hex 0a 0a 01 02 value hex $val_a_hex

    ok "Map populated."
    sudo bpftool map dump pinned "$map_pin"
}

# ---------------------------------------------------------------------------
# Step 5: Static ARP (neighbor) entries — bypass ARP discovery only
# ---------------------------------------------------------------------------
setup_static_arp() {
    log "Adding static ARP entries (the two veth pairs have no shared broadcast domain, so ARP can't resolve on its own)..."

    local mac_a mac_b

    mac_a=$(sudo ip netns exec "$NS_A" cat "/sys/class/net/${VETH_A_PEER}/address")
    mac_b=$(sudo ip netns exec "$NS_B" cat "/sys/class/net/${VETH_B_PEER}/address")

    sudo ip netns exec "$NS_A" ip neigh replace "$IP_B" lladdr "$mac_b" dev "$VETH_A_PEER" nud permanent
    sudo ip netns exec "$NS_B" ip neigh replace "$IP_A" lladdr "$mac_a" dev "$VETH_B_PEER" nud permanent

    ok "Static ARP entries set."
}

# ---------------------------------------------------------------------------
# Step 6: Proof there is no other forwarding path (bridge/route) at play
# ---------------------------------------------------------------------------
prove_no_other_path() {
    log "Confirming no bridge or route joins the two host-side veths..."

    ip -brief link show "$VETH_A_HOST"
    ip -brief link show "$VETH_B_HOST"

    if ip route show "$SUBNET" | grep -q .; then
        fail "Unexpected route for ${SUBNET} exists — result would be inconclusive."
        exit 1
    fi
    echo "    No route for ${SUBNET} (as expected)."

    if bridge link show | grep -qE "$VETH_A_HOST|$VETH_B_HOST"; then
        fail "Unexpected bridge membership for the veth-*-host interfaces — result would be inconclusive."
        exit 1
    fi
    echo "    No bridge membership for veth-*-host interfaces (as expected)."

    ok "No alternate forwarding path exists."
}

# ---------------------------------------------------------------------------
# Step 7: The actual test — real ping from ns-a to ns-b over the XDP path
# ---------------------------------------------------------------------------
run_ping_test() {
    log "Running ping test: ${NS_A} (${IP_A}) -> ${IP_B} via XDP redirect..."

    if sudo ip netns exec "$NS_A" ping -c "$PING_COUNT" -W 2 "$IP_B"; then
        ok "PASS: ping succeeded — forward and return paths both traversed the XDP redirect."
        return 0
    else
        fail "Ping did not succeed."
        return 1
    fi
}

# ---------------------------------------------------------------------------
# Debug: dump extra state to help diagnose a failing test
# ---------------------------------------------------------------------------
debug_dump() {
    log "Collecting debug info..."

    echo "--- ip link (root ns) ---"
    ip -brief link show || true

    echo "--- XDP attachment status ---"
    sudo bpftool net show || true

    echo "--- pod_interface_map contents ---"
    local map_pin
    map_pin="${PIN_MAPS_DIR}/$(sudo ls "$PIN_MAPS_DIR" 2>/dev/null || true)"
    sudo bpftool map dump pinned "$map_pin" 2>/dev/null || echo "    (map not available)"

    echo "--- ns-a interfaces / neighbors ---"
    sudo ip netns exec "$NS_A" ip addr show || true
    sudo ip netns exec "$NS_A" ip neigh show || true

    echo "--- ns-b interfaces / neighbors ---"
    sudo ip netns exec "$NS_B" ip addr show || true
    sudo ip netns exec "$NS_B" ip neigh show || true
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    check_prerequisites
    setup_namespaces
    compile_bpf
    load_and_attach_bpf
    populate_map
    setup_static_arp
    prove_no_other_path

    if run_ping_test; then
        log "E2E TEST PASSED"
    else
        log "E2E TEST FAILED — dumping debug info before cleanup"
        debug_dump
        exit 1
    fi
    # cleanup() runs automatically via the EXIT trap.
}

main "$@"
