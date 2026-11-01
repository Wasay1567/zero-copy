package network

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func buildDefaultRoute(linkIndex int, gateway net.IP) (*netlink.Route, error) {
	if linkIndex <= 0 {
		return nil, fmt.Errorf("invalid link index %d", linkIndex)
	}
	if gateway == nil {
		return nil, fmt.Errorf("gateway IP is required")
	}
	if gateway.To4() == nil {
		return nil, fmt.Errorf("gateway %q is not an IPv4 address", gateway)
	}

	_, defaultRoute, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		return nil, fmt.Errorf("failed to parse default route CIDR: %w", err)
	}

	return &netlink.Route{
		LinkIndex: linkIndex,
		Dst:       defaultRoute,
		Gw:        gateway.To4(),
	}, nil
}

func AttachGwRoute(netnsPath string, ifName string, gateway net.IP, gatewayMAC net.HardwareAddr) error {
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

		gwRoute, err := buildDefaultRoute(containerLink.Attrs().Index, gateway)
		if err != nil {
			return err
		}
		if err := netlink.RouteAdd(gwRoute); err != nil {
			if !errors.Is(err, syscall.EEXIST) {
				return fmt.Errorf("failed to add default route inside pod: %v", err)
			}
		}
		if len(gatewayMAC) != 6 {
			return fmt.Errorf("gateway MAC must contain 6 bytes")
		}
		if err := netlink.NeighSet(&netlink.Neigh{
			LinkIndex:    containerLink.Attrs().Index,
			IP:           gateway.To4(),
			HardwareAddr: gatewayMAC,
			State:        netlink.NUD_PERMANENT,
		}); err != nil {
			return fmt.Errorf("failed to add gateway neighbor inside pod: %v", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed configuring inside container netns: %v", err)
	}

	return nil
}
