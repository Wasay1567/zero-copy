package k8sHandler

import (
	"context"
	"fmt"

	"github.com/Wasay1567/zero-copy/daemon/bgpHandler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// NewNodeHandler returns event handlers closure-bound to the provided BGP server instance
func NewNodeHandler(g *bgpHandler.BGP) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ctx := context.Background()
			node := obj.(*corev1.Node)
			podCIDR := node.Spec.PodCIDR
			name := node.Name
			fmt.Printf("[Add] Node: %s, CIDR: %s\n", name, podCIDR)

			// Call your BGP methods using 'g'
			g.AddNeighbor(ctx, podCIDR, name)
		},
		DeleteFunc: func(obj interface{}) {
			ctx := context.Background()
			node := obj.(*corev1.Node)
			name := node.Name

			fmt.Printf("[Delete] Node: %s\n", name)

			g.RemoveNeighbor(ctx, node.Spec.PodCIDR)
		},
	}
}
