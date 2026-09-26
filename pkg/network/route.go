package network

import (
	"fmt"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func AttachGwRoute(netnsPath string, ifName string) error {
	// 1. Get a reference to the container's network namespace
	targetNS, err := ns.GetNS(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer targetNS.Close()

	err = targetNS.Do(func(hostNS ns.NetNS) error {
		containerLink, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to find interface inside container: %v", err)
		}

		_, defaultgw, _ := net.ParseCIDR("0.0.0.0/0")
		gwRoute := &netlink.Route{
			LinkIndex: containerLink.Attrs().Index,
			Dst:       defaultgw,
			Gw:        net.ParseIP("10.244.1.0"), // gateway ip for the host 
			// TODO: NEED TO ASSIGN IP THE HOST LINK
		}
		if err := netlink.RouteAdd(gwRoute); err != nil {
			return fmt.Errorf("failed to add default route inside pod: %v", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed configuring inside container netns: %v", err)
	}

	return nil
}

