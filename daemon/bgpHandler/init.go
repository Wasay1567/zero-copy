package bgpHandler

import (
	"context"
	"log"

	"github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/server"
)

type BGP struct {
	Server *server.BgpServer
	ASN    int
	Port   int
}

func InitBgp() *BGP {
	s := server.NewBgpServer()
	go s.Serve()
	defer s.Stop()

	if err := s.StartBgp(context.Background(), &api.StartBgpRequest{
		Global: &api.Global{
			Asn:        65001,
			RouterId:   "10.0.0.1",
			ListenPort: 179,
		},
	}); err != nil {
		log.Fatal(err)
		return nil
	}
	return &BGP{
		Server: s,
		ASN:    65001,
		Port:   179,
	}
}
