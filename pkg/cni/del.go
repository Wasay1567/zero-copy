package cni

import (
	"context"
	"errors"
	"os"

	"github.com/Wasay1567/zero-copy/pkg/ebpf"
	"github.com/Wasay1567/zero-copy/pkg/helpers"
	"github.com/Wasay1567/zero-copy/pkg/network"
	"github.com/containernetworking/cni/pkg/skel"
	goipam "github.com/metal-stack/go-ipam"
)

func Del(args *skel.CmdArgs) error {
	conf, err := loadConf(&skel.CmdArgs{StdinData: args.StdinData, Netns: "optional"})
	if err != nil && conf == nil {
		// DEL may be called after the netns has already disappeared, but the
		// network configuration is still required to locate the IPAM state.
		return err
	}
	ctx := context.Background()
	ipam, err := network.NewIPAM(ctx, goipam.New(ctx), conf.IPAM.Subnet, conf.IPAM.StateDir)
	if err != nil {
		return err
	}
	defer ipam.Close()

	ip, found, err := ipam.AllocatedIP(args.ContainerID, args.IfName)
	if err != nil {
		return err
	}
	if found {
		if err := ebpf.DeletePodFromFastPath(ip.String()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	hostVeth, err := helpers.BuildNameForHostVeth(args.ContainerID)
	if err != nil {
		return err
	}
	if err := network.DeleteVeth(hostVeth); err != nil {
		return err
	}
	return ipam.ReleaseIP(ctx, args.ContainerID, args.Netns, args.IfName)
}
