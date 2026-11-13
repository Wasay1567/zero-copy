package bgpHandler

import (
	"context"
	"fmt"

	"github.com/osrg/gobgp/v4/api"
)

// AddNeighbor configures a GoBGP peer for a remote node and labels it with the node name.
func (b *BGP) AddNeighbor(ctx context.Context, podCIDR string, nodeName string) error {
	// neighbor configuration
	n := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: podCIDR,
			PeerAsn:         uint32(b.ASN),
			Description:     nodeName,
		},
	}
	if err := b.Server.AddPeer(ctx, &api.AddPeerRequest{Peer: n}); err != nil {
		return fmt.Errorf("failed to add peer %s, err: %v", podCIDR, err)
	}
	return nil
}

// RemoveNeighbor deletes a GoBGP peer identified by its address.
func (b *BGP) RemoveNeighbor(ctx context.Context, podCIDR string) error {
	// neighbor configuration
	n := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: podCIDR,
			PeerAsn:         uint32(b.ASN),
		},
	}
	if err := b.Server.DeletePeer(ctx, &api.DeletePeerRequest{Address: n.Conf.NeighborAddress}); err != nil {
		return fmt.Errorf("failed to remove peer %s, err: %v", podCIDR, err)
	}
	return nil
}
