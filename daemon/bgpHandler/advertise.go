package bgpHandler

// import (
// 	"context"
// 	"path"

// 	api "github.com/osrg/gobgp/v3/api"
// 	"github.com/osrg/gobgp/v3/pkg/apiutil"
// 	"google.golang.org/protobuf/types/known/anypb"
// )

// func (b *BGP) Advertise(prefixLen uint32, podCIDR, nodeIP string) {
// 	// 1. Define NLRI (Network Layer Reachability Information) for your local podCIDR
// 	nlri, _ := anypb.New(&api.IPAddressPrefix{
// 		PrefixLen: prefixLen,
// 		Prefix:    podCIDR, // Local Node podCIDR
// 	})

// 	// 2. Set NextHop to local physical Node IP
// 	nextHop, _ := anypb.New(&api.NextHopAttribute{
// 		NextHop: nodeIP, // Local Node IP
// 	})

// 	origin, _ := anypb.New(&api.OriginAttribute{
// 		Origin: 0, // IGP
// 	})

// 	path := &api.Path{
// 		Family: &api.Family{
// 			Afi:  api.Family_AFI_IP,
// 			Safi: api.Family_SAFI_UNICAST,
// 		},
// 		Nlri:   nlri,
// 		Pattrs: []*anypb.Any{origin, nextHop},
// 	}

// 	// 3. Add Path to GoBGP
// 	_, _ = b.Server.AddPath(context.Background(), apiutil.AddPathRequest{
// 		TableType: api.TableType_GLOBAL,
// 		Path: path,
// 	})
// }
