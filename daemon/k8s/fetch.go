package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type plugins struct {
	Type string `json:"type"`
	IPAM struct {
		Type   string `json:"type"`
		Subnet string `json:"subnet"`
	} `json:"ipam"`
}

type CNICONFIG struct {
	CNI_Version string    `json:"cniVersion"`
	Name        string    `json:"name"`
	Plugins     []plugins `json:"plugins"`
}

// Source - https://stackoverflow.com/a/57791506
// Posted by ZuBsPaCe
// Retrieved 2026-10-03, License - CC BY-SA 4.0

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func FetchPodCIDR(nodeName string) (string, error) {
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

	// 3. Query the specific Node resource from the API Server
	// Nodes are cluster-scoped, so no namespace is provided
	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("failed to get node details from API server: %v\n", err)
		os.Exit(1)
	}

	// 4. Extract and print the PodCIDR
	podCIDR := node.Spec.PodCIDR
	if podCIDR == "" {
		fmt.Println("Warning: Spec.PodCIDR is empty. Ensure your cluster CNI uses Kubernetes IPAM.")
	} else {
		fmt.Printf("Success! Local Node %s PodCIDR: %s\n", nodeName, podCIDR)
	}

	return podCIDR, nil
}

func SavePodCIDRToFile(podCIDR, filePath string) error {

	if fileExists(filePath) {
		fmt.Printf("File already exists, skipping write to file: %s\n", filePath)
		return nil
	}

	data := &CNICONFIG{
		CNI_Version: "1.0.0",
		Name:        "zero-copy-ebpf-cni",
		Plugins: []plugins{
			{
				Type: "my-cni-plugin",
				IPAM: struct {
					Type   string `json:"type"`
					Subnet string `json:"subnet"`
				}{
					Type:   "host-local",
					Subnet: podCIDR,
				},
			},
		},
	}

	d, err := json.Marshal(&data)
	if err != nil {
		return fmt.Errorf("failed to marshal CNICONFIG to JSON: %v", err)
	}

	err = os.WriteFile(filePath, d, 0644)
	if err != nil {
		return fmt.Errorf("failed to write CNICONFIG to file: %v", err)
	}

	fmt.Printf("Successfully wrote CNICONFIG to %s\n", filePath)
	return nil
}
