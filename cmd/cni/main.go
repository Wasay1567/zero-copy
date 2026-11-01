package main

import (
	"github.com/Wasay1567/zero-copy/pkg/cni"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
)

func main() {
	skel.PluginMain(cni.Add, func(*skel.CmdArgs) error { return nil }, cni.Del,
		version.PluginSupports("0.3.0", "0.3.1", "0.4.0", "1.0.0", "1.1.0"),
		"zero-copy CNI plugin")
}