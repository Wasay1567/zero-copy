package bgpHandler

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/osrg/gobgp/v4/pkg/apiutil"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/osrg/gobgp/v4/pkg/server"
	"github.com/vishvananda/netlink"
)

// getAttr finds and type-asserts the first matching BGP PathAttribute from p.Attrs.
func getAttr[T bgp.PathAttributeInterface](attrs []bgp.PathAttributeInterface) (T, bool) {
	for _, a := range attrs {
		if v, ok := a.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// Monitor watches GoBGP path updates and synchronizes them with host routes.
func (b *BGP) Monitor(ctx context.Context) error {
	if err := b.Server.WatchEvent(ctx, server.WatchEventMessageCallbacks{
		OnPathUpdate: func(paths []*apiutil.Path, t time.Time) {
			for _, p := range paths {
				// 1. Get podCIDR directly from NLRI (returns netip.Prefix)
				ipPrefix, ok := p.Nlri.(*bgp.IPAddrPrefix)
				if !ok {
					continue // Not an IPv4/IPv6 unicast prefix
				}
				remoteCIDR := ipPrefix.Prefix // netip.Prefix (e.g., 10.244.1.0/24)

				var nodeIP netip.Addr
				if nh, ok := getAttr[*bgp.PathAttributeNextHop](p.Attrs); ok {
					nodeIP = nh.Value // netip.Addr
				}

				if p.Withdrawal {
					if err := addHostRoute(remoteCIDR, nodeIP); err != nil {
						fmt.Println(err)
					}
				} else {
					if err := removeHostRoute(remoteCIDR, nodeIP); err != nil {
						fmt.Println(err)
					}
				}
			}
		},
	}); err != nil {
		return err
	}
	return nil
}

// addHostRoute installs or replaces a route to a remote pod CIDR through its next hop.
func addHostRoute(remoteCIDR netip.Prefix, nextHopIP netip.Addr) error {
	link, err := netlink.LinkByName("eth0")
	if err != nil {
		return err
	}

	addr := remoteCIDR.Addr()
	dst := &net.IPNet{
		IP:   net.IP(addr.AsSlice()),
		Mask: net.CIDRMask(remoteCIDR.Bits(), addr.BitLen()),
	}

	r := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Gw:        net.ParseIP(nextHopIP.String()),
	}
	err = netlink.RouteReplace(r)
	if err != nil {
		return fmt.Errorf("failed to add route for dst %v to this gw %v : %v", r.Dst, r.Gw, err)
	}
	return nil
}

// removeHostRoute removes the host route to a remote pod CIDR through its next hop.
func removeHostRoute(remoteCIDR netip.Prefix, nextHopIP netip.Addr) error {
	link, err := netlink.LinkByName("eth0")
	if err != nil {
		return err
	}

	addr := remoteCIDR.Addr()
	dst := &net.IPNet{
		IP:   net.IP(addr.AsSlice()),
		Mask: net.CIDRMask(remoteCIDR.Bits(), addr.BitLen()),
	}

	r := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Gw:        net.ParseIP(nextHopIP.String()),
	}
	err = netlink.RouteDel(r)
	if err != nil {
		return fmt.Errorf("failed to delete route for dst %v to this gw %v : %v", r.Dst, r.Gw, err)
	}
	return nil
}
