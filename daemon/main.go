package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"time"

	"github.com/Wasay1567/zero-copy/daemon/bgpHandler"
	"github.com/Wasay1567/zero-copy/daemon/k8sHandler"
	"github.com/Wasay1567/zero-copy/pkg/ebpf"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// main loads the eBPF agent, configures the local CNI state, and optionally starts the BGP controller.
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	objectPath := os.Getenv("EBPF_OBJECT")
	if objectPath == "" {
		objectPath = "/opt/zero-copy/fast-path.o"
	}
	agent, err := ebpf.LoadAgent(objectPath)
	if err != nil {
		fmt.Printf("failed to load XDP agent: %v\n", err)
		os.Exit(1)
	}
	defer agent.Close()
	go func() {
		if err := agent.Run(ctx); err != nil {
			fmt.Printf("XDP agent stopped: %v\n", err)
			cancel()
		}
	}()

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

	if os.Getenv("ENABLE_BGP") == "true" {
		g := bgpHandler.InitBgp()
		if g == nil {
			os.Exit(1)
		}
		stopper := make(chan struct{})
		defer close(stopper)
		factory := informers.NewSharedInformerFactoryWithOptions(clientset, 10*time.Second)
		nodeInformer := factory.Core().V1().Nodes().Informer()
		defer runtime.HandleCrash()
		go factory.Start(stopper)
		if !cache.WaitForCacheSync(stopper, nodeInformer.HasSynced) {
			runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
			return
		}
		k8sHandler.NewNodeHandler(g)
		go g.Monitor(ctx)

	}

	// block the main go routine from exiting
	<-ctx.Done()

}
