package network

// func DeleteVeth() {

// }

import (
	"fmt"
	"os"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

const DEFAULT_CIDR = "10.244.0.0/16"

// setupVeth creates a veth pair, moves one end into the container namespace,
// and the host end remains hanging .
func SetupVeth(netnsPath string, ifName string, bridgeName string) error {
	// 1. Get a reference to the container's network namespace
	targetNS, err := ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer targetNS.Close()

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

	// Bring the host-side veth interface UP (It sits loose in root namespace)
	if err := netlink.LinkSetUp(vethLink); err != nil {
		return fmt.Errorf("failed to bring host veth up: %v", err)
	}

	// 5. Look up the container-side peer interface we just made
	peerLink, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to find peer veth: %v", err)
	}

	// 6. Move the container-side peer into the target Pod's network namespace
	if err := netlink.LinkSetNsFd(peerLink, int(targetNS.Fd())); err != nil {
		return fmt.Errorf("failed to move veth into container netns: %v", err)
	}

	// 7. Execute code *inside* the container's namespace to finalize configuration
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

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed configuring inside container netns: %v", err)
	}

	return nil
}



func DeleteVeth(hostVethName string) error {
	link, err := netlink.LinkByName(hostVethName)
	if err != nil {
		// If link not found then it must be deleted
		return nil
	}

	// Delete the host-side interface
	// This single call destroys both sides of the veth pair and cleans up the pod's routes
	err = netlink.LinkDel(link)
	if err != nil {
		return fmt.Errorf("failed to delete host veth %s:  %v", hostVethName, err)
	}

	return nil
}

func main() {
	// In a real CNI, these values are parsed from Env Variables passed by Kubelet
	netns := os.Getenv("CNI_NETNS")   // Path to container netns file (e.g., /proc/\$PID/ns/net)
	ifname := os.Getenv("CNI_IFNAME") // Name of the interface to create (usually "eth0")
	bridge := "cni0"                  // Your single per-node bridge

	if err := SetupVeth(netns, ifname, bridge); err != nil {
		fmt.Fprintf(os.Stderr, "Error running custom CNI: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(`{"cniVersion": "0.4.0", "interfaces": [{"name": "eth0"}]}`)
}
