package bgpHandler

import (
	"net/netip"

	"github.com/osrg/gobgp/v4/pkg/apiutil"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
)

// Advertise publishes a pod CIDR through GoBGP with the node IP as its next hop.
func (b *BGP) Advertise(prefixLen uint32, podCIDR, nodeIP string) {
	// 1. Define NLRI using the packet bgp helper
	prefix := netip.MustParsePrefix(podCIDR)
	nlri, _ := bgp.NewIPAddrPrefix(prefix)

	// 2. Set Origin and NextHop attributes
	origin := bgp.NewPathAttributeOrigin(0) // IGP
	nodeAddr := netip.MustParseAddr(nodeIP)
	nextHop, _ := bgp.NewPathAttributeNextHop(nodeAddr)

	// 3. Add Path to GoBGP using apiutil.AddPathRequest (passed by value, no context)
	_, err := b.Server.AddPath(apiutil.AddPathRequest{
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
	if err != nil {
		// Handle advertisement error
	}
}
