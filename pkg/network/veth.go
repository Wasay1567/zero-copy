package network

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Wasay1567/zero-copy/pkg/helpers"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

// isGone reports whether err means "link or netns does not exist".
func isGone(err error) bool {
	var lnf netlink.LinkNotFoundError
	var nsErr ns.NSPathNotExistErr
	return errors.As(err, &lnf) || errors.As(err, &nsErr)
}

// setupVeth creates a veth pair, moves one end into the container namespace,
// and the host end remains hanging .
func SetupVeth(netnsPath string, ifName string, containerID string) (string, error) {
	// 1. Get a reference to the container's network namespace
	targetNS, err := ns.GetNS(netnsPath)
	if err != nil {
		return "", fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer targetNS.Close()

	hostVethName, err := helpers.BuildNameForHostVeth(containerID)
	if err != nil {
		return "", fmt.Errorf("failed to build host veth name: %v", err)
	}
	peerName := "peer-" + strings.TrimPrefix(hostVethName, "veth-")

	// Create the peer with a temporary host-namespace name. The target
	// namespace owns the final interface name, usually "eth0".
	vethLink := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostVethName,
		},
		PeerName: peerName,
	}

	// 4. Create the veth pair on the host
	if err := netlink.LinkAdd(vethLink); err != nil {
		return "", fmt.Errorf("failed to create veth pair: %v", err)
	}
	cleanupVeth := true
	defer func() {
		if cleanupVeth {
			_ = DeleteVeth(hostVethName)
		}
	}()

	// Bring the host-side veth interface UP (It sits loose in root namespace)
	if err := netlink.LinkSetUp(vethLink); err != nil {
		return "", fmt.Errorf("failed to bring host veth up: %v", err)
	}

	// 5. Look up the container-side peer interface we just made
	peerLink, err := netlink.LinkByName(peerName)
	if err != nil {
		return "", fmt.Errorf("failed to find peer veth: %v", err)
	}

	// 6. Move the container-side peer into the target Pod's network namespace
	if err := netlink.LinkSetNsFd(peerLink, int(targetNS.Fd())); err != nil {
		return "", fmt.Errorf("failed to move veth into container netns: %v", err)
	}

	// 7. Execute code *inside* the container's namespace to finalize configuration
	err = targetNS.Do(func(hostNS ns.NetNS) error {
		// Fetch the interface again, now that we are context-shifted inside the pod ns
		containerLink, err := netlink.LinkByName(peerName)
		if err != nil {
			return fmt.Errorf("failed to find interface inside container: %v", err)
		}
		if err := netlink.LinkSetName(containerLink, ifName); err != nil {
			return fmt.Errorf("failed to rename interface to %q: %v", ifName, err)
		}
		containerLink, err = netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to find renamed interface %q: %v", ifName, err)
		}

		// Bring the interface UP inside the pod
		if err := netlink.LinkSetUp(containerLink); err != nil {
			return fmt.Errorf("failed to bring container interface up: %v", err)
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed configuring inside container netns: %v", err)
	}

	cleanupVeth = false
	return hostVethName, nil
}

func DeleteVeth(hostVethName string) error {
	link, err := netlink.LinkByName(hostVethName)
	if err != nil {
		if isGone(err) {
			return nil
		}
		return fmt.Errorf("find host veth %q: %w", hostVethName, err)
	}

	if _, ok := link.(*netlink.Veth); !ok {
		return fmt.Errorf("refusing to delete %q: not a veth", hostVethName)
	}

	// Delete the host-side interface
	// This single call destroys both sides of the veth pair and cleans up the pod's routes
	err = netlink.LinkDel(link)
	if err != nil {
		return fmt.Errorf("failed to delete host veth %s:  %v", hostVethName, err)
	}

	return nil
}
