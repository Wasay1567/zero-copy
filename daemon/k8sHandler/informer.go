package k8sHandler

import (
	"context"
	"fmt"
	"net"

	"github.com/Wasay1567/zero-copy/daemon/bgpHandler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// NewNodeHandler returns event handlers that add and remove Kubernetes node BGP peers.
func NewNodeHandler(g *bgpHandler.BGP) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ctx := context.Background()
			node := obj.(*corev1.Node)
			nodeIP := getReachableIPAddress(node)
			if nodeIP == "" {
				return
			}
			name := node.Name
			fmt.Printf("[Add] Node: %s, with reachable IP %s\n", name, nodeIP)

			// Call your BGP methods using 'g'
			g.AddNeighbor(ctx, nodeIP, name)
		},
		DeleteFunc: func(obj interface{}) {
			ctx := context.Background()
			node := obj.(*corev1.Node)
			name := node.Name
			nodeIP := getReachableIPAddress(node)
			if nodeIP == "" {
				return
			}
			fmt.Printf("[Delete] Node: %s\n", name)


			g.RemoveNeighbor(ctx, nodeIP)
		},
	}
}


func getReachableIPAddress(node *corev1.Node) string {
	var internalIP string 

	for _, addr := range node.Status.Addresses {
		ip := net.ParseIP(addr.Address)
		if ip == nil || ip.To4() == nil {
			continue
		}
		switch addr.Type {
		case corev1.NodeExternalIP:
			return addr.Address
		case corev1.NodeInternalIP:
			internalIP = addr.Address
		}
	}
	return internalIP
}