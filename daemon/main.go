package main

import (
	"fmt"
	"os"

	"time"

	"github.com/Wasay1567/zero-copy/daemon/bgpHandler"
	"github.com/Wasay1567/zero-copy/daemon/k8sHandler"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func main() {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		fmt.Println("Error: NODE_NAME environment variable is not set.")
		fmt.Println("Ensure spec.nodeName is mapped via fieldRef in your DaemonSet manifest.")
		os.Exit(1)
	}

	// 1. Configure the client using the Pod's internal ServiceAccount token
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("failed to create in-cluster config: %v\n", err)
		os.Exit(1)
	}

	// 2. Create the Kubernetes Clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("failed to create kubernetes clientset: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Fetching PodCIDR for current host node: %s\n", nodeName)
	cidr, err := k8sHandler.FetchPodCIDR(nodeName, clientset)
	if err != nil {
		fmt.Printf("Error occured while fetching the pod cidr of node %s: %v\n", nodeName, err)
		os.Exit(1)
	}
	if err := k8sHandler.SavePodCIDRToFile(cidr, "/host/etc/cni/net.d/10-zerocopy.conflist"); err != nil {
		fmt.Printf("Error occured while saving the pod cidr of node %s: %v\n", nodeName, err)
		os.Exit(1)
	}

	// Initialize new bgp server on port 179
	g := bgpHandler.InitBgp()
	if g == nil {
		os.Exit(1)
	}

	// stop signal for the informer
	stopper := make(chan struct{})
	defer close(stopper)

	// create shared informers for resources in all known API group versions with a reSync period and namespace
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 10*time.Second)
	NodeInformer := factory.Core().V1().Nodes().Informer()

	defer runtime.HandleCrash()

	// start informer ->
	go factory.Start(stopper)

	// start to sync and call list
	if !cache.WaitForCacheSync(stopper, NodeInformer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	k8sHandler.NewNodeHandler(g)

	// block the main go routine from exiting
	<-stopper

}
