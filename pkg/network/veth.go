package network

// func DeleteVeth() {

// }

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"

	"github.com/containernetworking/plugins/pkg/ns"
	goipam "github.com/metal-stack/go-ipam"
	"github.com/vishvananda/netlink"
)

const DEFAULT_CIDR = "10.24.0.0/16"

// setupVeth creates a veth pair, moves one end into the container namespace,
// and connects the host end to a local bridge.
func setupVeth(netnsPath string, ifName string, bridgeName string) error {
	// 1. Get a reference to the container's network namespace
	targetNS, err := ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer targetNS.Close()

	// 2. Find the host's bridge interface (e.g., "cni0")
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("failed to find bridge %q: %v", bridgeName, err)
	}

	hostVethName := "veth-" + os.Getenv("CNI_CONTAINERID")[:8] // Unique host-side name

	// 3. Define the veth pair configuration
	vethLink := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostVethName,
		},
		PeerName: ifName, // This will be renamed to "eth0" inside the container
	}

	// 4. Create the veth pair on the host
	if err := netlink.LinkAdd(vethLink); err != nil {
		return fmt.Errorf("failed to create veth pair: %v", err)
	}

	// 5. Connect the host-side of the veth to your node's bridge
	if err := netlink.LinkSetMaster(vethLink, br.(*netlink.Bridge)); err != nil {
		return fmt.Errorf("failed to connect %s to bridge: %v", hostVethName, err)
	}

	// Bring the host-side veth interface UP
	if err := netlink.LinkSetUp(vethLink); err != nil {
		return fmt.Errorf("failed to bring host veth up: %v", err)
	}

	// 6. Look up the container-side peer interface we just made
	peerLink, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to find peer veth: %v", err)
	}

	// 7. Move the container-side peer into the target Pod's network namespace
	if err := netlink.LinkSetNsFd(peerLink, int(targetNS.Fd())); err != nil {
		return fmt.Errorf("failed to move veth into container netns: %v", err)
	}

	// 8. Execute code *inside* the container's namespace to finalize configuration
	err = targetNS.Do(func(hostNS ns.NetNS) error {
		// Fetch the interface again, now that we are context-shifted inside the pod ns
		containerLink, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to find interface inside container: %v", err)
		}

		// Bring the interface UP inside the pod
		if err := netlink.LinkSetUp(containerLink); err != nil {
			return fmt.Errorf("failed to bring container interface up: %v", err)
		}

		// Optional: Parse and assign an IP address here using netlink.AddrAdd()
		// For example:
		// ip, ipNet, _ := net.ParseCIDR("10.244.1.5/24")
		// addr := &netlink.Addr{IPNet: &net.IPNet{IP: ip, Mask: ipNet.Mask}}
		// netlink.AddrAdd(containerLink, addr)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed configuring inside container netns: %v", err)
	}

	return nil
}

func allocateIP(netnsPath string, ifName string) error {
	targetNS, err := ns.Getns(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer targetNS.Close()

	ctx := context.Background()
	ipam := goipam.New(ctx)

	// Create a prefix to manage some IPs
	prefix, err := ipam.NewPrefix(ctx, DEFAULT_CIDR)
	if err != nil {
		panic(err) // FIX
	}

	ip, err := ipam.AcquireIP(ctx, prefix.Cidr)
	if err != nil {
		panic(err) // FIX
	}

	parsedIP := net.ParseIP(ip.IP.String())

	err = targetNS.Do(func(hostNS ns.NetNS) error {
		// Fetch the interface again, now that we are context-shifted inside the pod ns
		containerLink, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to find interface inside container: %v", err)
		}

		addr := &netlink.Addr{IPNet: &net.IPNet{IP: parsedIP, Mask: net.CIDRMask(16, 32)}}
		err = netlink.AddrAdd(containerLink, addr)
		if err != nil {
			return fmt.Errorf("failed to assign ip address %")
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed configuring inside container netns: %v", err)
	}

	return nil
}

func releaseIP(netnsPath string, ifName string, ipam goipam.Ipamer, ctx context.Context) error {
	targetNS, err := ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer targetNS.Close()

	nlHandle, err := netlink.NewHandleAt(targetNS)
	if err != nil {
		return fmt.Errorf("Failed to create netlink handle: %v", err)
	}
	defer nlHandle.Delete()

	containerLink, err := nlHandle.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to find interface inside container: %v", err)
	}

	addrs, err := nlHandle.AddrList(containerLink, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("Failed to list addresses: %v", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("interface %q has no IPv4 addresses", ifName)
	}

	address := addrs[0]
	ip, err := netip.ParseAddr(address.IPNet.IP.String())
	if err != nil {
		return fmt.Errorf("failed to parse address %q: %v", address.IPNet.IP, err)
	}

	if err := nlHandle.AddrDel(containerLink, &address); err != nil {
		return fmt.Errorf("failed to remove IP address from container link: %v", err)
	}

	if _, err := ipam.ReleaseIP(ctx, &goipam.IP{IP: ip, ParentPrefix: DEFAULT_CIDR}); err != nil {
		return fmt.Errorf("failed to release IP %s from IPAM: %v", ip, err)
	}

	return nil
}

func deleteVeth(netnsPath string, ifName string) {

}

func main() {
	// In a real CNI, these values are parsed from Env Variables passed by Kubelet
	netns := os.Getenv("CNI_NETNS")   // Path to container netns file (e.g., /proc/\$PID/ns/net)
	ifname := os.Getenv("CNI_IFNAME") // Name of the interface to create (usually "eth0")
	bridge := "cni0"                  // Your single per-node bridge

	if err := setupVeth(netns, ifname, bridge); err != nil {
		fmt.Fprintf(os.Stderr, "Error running custom CNI: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(`{"cniVersion": "0.4.0", "interfaces": [{"name": "eth0"}]}`)
}
