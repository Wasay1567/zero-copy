package helpers

import "fmt"

func BuildNameForHostVeth(containerID string) (string, error) {
	if containerID == "" {
		return "", fmt.Errorf("container ID is required")
	}
	if len(containerID) > 8 {
		containerID = containerID[:8]
	}
	return "veth-" + containerID, nil
}
