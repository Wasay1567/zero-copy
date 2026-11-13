package bgpHandler

import (
	"context"
	"fmt"

	"github.com/osrg/gobgp/v4/api"
)

// Reference: https://github.com/osrg/gobgp/blob/master/docs/sources/lib.md

// AddNeighbor configures a GoBGP peer for a remote node and labels it with the node name.
func (b *BGP) AddNeighbor(ctx context.Context, nodeIP string, nodeName string) error {
	// neighbor configuration
	n := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: nodeIP,
			PeerAsn:         uint32(b.ASN),
			Description:     nodeName,
		},
	}
	if err := b.Server.AddPeer(ctx, &api.AddPeerRequest{Peer: n}); err != nil {
		return fmt.Errorf("failed to add peer %s, err: %v", nodeIP, err)
	}
	return nil
}

// RemoveNeighbor deletes a GoBGP peer identified by its address.
func (b *BGP) RemoveNeighbor(ctx context.Context, nodeIP string) error {
	// neighbor configuration
	n := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: nodeIP,
			PeerAsn:         uint32(b.ASN),
		},
	}
	if err := b.Server.DeletePeer(ctx, &api.DeletePeerRequest{Address: n.Conf.NeighborAddress}); err != nil {
		return fmt.Errorf("failed to remove peer %s, err: %v", nodeIP, err)
	}
	return nil
}
