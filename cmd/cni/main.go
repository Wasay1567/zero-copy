package main

import (
	"github.com/Wasay1567/zero-copy/pkg/cni"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
)

func main() {
	// Wire a real backend here once network/IPAM provisioning is implemented.
	handler := cni.NewHandler(nil)
	skel.PluginMainFuncs(handler.Funcs(), version.PluginSupports(cni.SupportedCNIVersion), "zero-copy CNI plugin")
}
