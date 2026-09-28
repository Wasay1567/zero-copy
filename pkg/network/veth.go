package network

// func DeleteVeth() {

// }

import (
	"errors"
	"fmt"
	"os"

	"github.com/Wasay1567/zero-copy/pkg/helpers"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

// setupVeth creates a veth pair, moves one end into the container namespace,
// and the host end remains hanging .
func SetupVeth(netnsPath string, ifName string, bridgeName string) error {
	// 1. Get a reference to the container's network namespace
	targetNS, err := ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer targetNS.Close()

	hostVethName, err := helpers.BuildNameForHostVeth(os.Getenv("CNI_CONTAINERID"))
	if err != nil {
		return fmt.Errorf("failed to build host veth name: %v", err)
	}

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
	if errors.Is(err, netlink.LinkNotFoundError{}) {
		// If link not found then it must be deleted
		return nil
	} else if err != nil {
		return fmt.Errorf("Could not find the host veth link: %v", err)
	}

	// Delete the host-side interface
	// This single call destroys both sides of the veth pair and cleans up the pod's routes
	err = netlink.LinkDel(link)
	if err != nil {
		return fmt.Errorf("failed to delete host veth %s:  %v", hostVethName, err)
	}

	return nil
}
