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
func NewNodeHandler(g *bgpHandler.BGP, localNodeName string) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ctx := context.Background()
			node, ok := nodeFromEvent(obj)
			if !ok || node.Name == localNodeName {
				return
			}
			nodeIP := getReachableIPAddress(node)
			if nodeIP == "" {
				return
			}
			name := node.Name
			fmt.Printf("[Add] Node: %s, with reachable IP %s\n", name, nodeIP)

			// Call your BGP methods using 'g'
			if err := g.AddNeighbor(ctx, nodeIP, name); err != nil {
				fmt.Println(err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			ctx := context.Background()
			node, ok := nodeFromEvent(obj)
			if !ok || node.Name == localNodeName {
				return
			}
			name := node.Name
			nodeIP := getReachableIPAddress(node)
			if nodeIP == "" {
				return
			}
			fmt.Printf("[Delete] Node: %s\n", name)

			if err := g.RemoveNeighbor(ctx, nodeIP); err != nil {
				fmt.Println(err)
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			ctx := context.Background()
			oldnode, oldOK := nodeFromEvent(oldObj)
			newnode, newOK := nodeFromEvent(newObj)
			if !oldOK || !newOK || oldnode.Name == localNodeName || newnode.Name == localNodeName {
				return
			}

			oldIP := getReachableIPAddress(oldnode)
			newIP := getReachableIPAddress(newnode)

			if oldIP == newIP {
				return
			}
			if oldIP != "" {
				if err := g.RemoveNeighbor(ctx, oldIP); err != nil {
					fmt.Printf("failed to remove old peer for node %s: %v\n", newnode.Name, err)
					return
				}
			}
			if newIP != "" {
				if err := g.AddNeighbor(ctx, newIP, newnode.Name); err != nil {
					fmt.Println(err)
				}
			}
		},
	}
}

// nodeFromEvent extracts a Node from a normal informer event or delete tombstone.
func nodeFromEvent(obj interface{}) (*corev1.Node, bool) {
	switch event := obj.(type) {
	case *corev1.Node:
		return event, true
	case cache.DeletedFinalStateUnknown:
		node, ok := event.Obj.(*corev1.Node)
		return node, ok
	case *cache.DeletedFinalStateUnknown:
		node, ok := event.Obj.(*corev1.Node)
		return node, ok
	default:
		return nil, false
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
