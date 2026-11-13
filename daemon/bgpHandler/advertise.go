package bgpHandler

import (
	"net/netip"

	"github.com/osrg/gobgp/v4/pkg/apiutil"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
)

// Reference: https://github.com/osrg/gobgp/blob/master/docs/sources/lib.md

// Advertise publishes a pod CIDR through GoBGP with the node IP as its next hop.
func (b *BGP) Advertise(podCIDR, nodeIP string) error {
	// 1. Define NLRI using the packet bgp helper
	prefix := netip.MustParsePrefix(podCIDR)
	nlri, err := bgp.NewIPAddrPrefix(prefix)
	if err != nil {
		return err
	}

	// 2. Set Origin and NextHop attributes
	origin := bgp.NewPathAttributeOrigin(0) // IGP
	nodeAddr := netip.MustParseAddr(nodeIP)
	nextHop, err := bgp.NewPathAttributeNextHop(nodeAddr)
	if err != nil {
		return err
	}


	// 3. Add Path to GoBGP using apiutil.AddPathRequest (passed by value, no context)
	_, err = b.Server.AddPath(apiutil.AddPathRequest{
		Paths: []*apiutil.Path{
			{
				Nlri: nlri,
				Attrs: []bgp.PathAttributeInterface{
					origin,
					nextHop,
				},
			},
		},
	})
	return err
}
