package cni

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/Wasay1567/zero-copy/pkg/ebpf"
	"github.com/Wasay1567/zero-copy/pkg/network"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	goipam "github.com/metal-stack/go-ipam"
	"github.com/vishvananda/netlink"
)

type NetConf struct {
	types.NetConf
	IPAM struct {
		Type     string `json:"type"`
		Subnet   string `json:"subnet"`
		StateDir string `json:"stateDir,omitempty"`
	} `json:"ipam"`
}

func loadConf(args *skel.CmdArgs) (*NetConf, error) {
	conf := &NetConf{}
	if err := json.Unmarshal(args.StdinData, conf); err != nil {
		return nil, fmt.Errorf("parse CNI config: %w", err)
	}
	if conf.CNIVersion == "" {
		return nil, fmt.Errorf("cniVersion is required")
	}
	if conf.IPAM.Subnet == "" {
		return nil, fmt.Errorf("ipam.subnet is required")
	}
	if args.Netns == "" {
		return nil, fmt.Errorf("CNI ADD requires a network namespace")
	}
	return conf, nil
}

func Add(args *skel.CmdArgs) error {
	conf, err := loadConf(args)
	if err != nil {
		return err
	}
	ctx := context.Background()
	ipam, err := network.NewIPAM(ctx, goipam.New(ctx), conf.IPAM.Subnet, conf.IPAM.StateDir)
	if err != nil {
		return err
	}
	defer ipam.Close()

	hostVeth, err := network.SetupVeth(args.Netns, args.IfName, args.ContainerID)
	if err != nil {
		return err
	}
	cleanupVeth := true
	defer func() {
		if cleanupVeth {
			_ = network.DeleteVeth(hostVeth)
		}
	}()

	ipnet, err := ipam.AllocateIP(ctx, args.ContainerID, args.Netns, args.IfName)
	if err != nil {
		return err
	}
	allocatedIP := true
	defer func() {
		if allocatedIP {
			_ = ipam.ReleaseIP(ctx, args.ContainerID, args.Netns, args.IfName)
		}
	}()
	gateway, err := ipam.Gateway()
	if err != nil {
		return err
	}
	if err := network.AttachGwRoute(args.Netns, args.IfName, gateway); err != nil {
		return err
	}

	hostLink, err := netlink.LinkByName(hostVeth)
	if err != nil {
		return fmt.Errorf("find host veth %q: %w", hostVeth, err)
	}
	if err := ebpf.AddPodToFastPath(ipnet.IP.String(), hostLink.Attrs().Index); err != nil {
		return fmt.Errorf("add pod to XDP fast path: %w", err)
	}
	allocatedIP = false
	cleanupVeth = false

	_, defaultRoute, _ := net.ParseCIDR("0.0.0.0/0")
	result := &current.Result{
		CNIVersion: conf.CNIVersion,
		Interfaces: []*current.Interface{{Name: args.IfName, Sandbox: args.Netns}},
		IPs:        []*current.IPConfig{{Address: *ipnet, Gateway: gateway, Interface: current.Int(0)}},
		Routes:     []*types.Route{{Dst: *defaultRoute, GW: gateway}},
	}
	return types.PrintResult(result, conf.CNIVersion)
}
