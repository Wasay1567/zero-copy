package main

import (
	"fmt"
	"os"

	"github.com/Wasay1567/zero-copy/daemon/k8s"
)

func main() {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		fmt.Println("Error: NODE_NAME environment variable is not set.")
		fmt.Println("Ensure spec.nodeName is mapped via fieldRef in your DaemonSet manifest.")
		os.Exit(1)
	}

	fmt.Printf("Fetching PodCIDR for current host node: %s\n", nodeName)
	cidr, err := k8s.FetchPodCIDR(nodeName)
	if err != nil {
		fmt.Printf("Error occured while fetching the pod cidr of node %s: %v\n", nodeName, err)
		os.Exit(1)
	}
	if err := k8s.SavePodCIDRToFile(cidr, "/host/etc/cni/net.d/10-zerocopy.conflist"); err != nil {
		fmt.Printf("Error occured while saving the pod cidr of node %s: %v\n", nodeName, err)
		os.Exit(1)
	}

	
}
