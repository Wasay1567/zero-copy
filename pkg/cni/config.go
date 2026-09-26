package cni

import (
	"encoding/json"
	"errors"
	"fmt"
)

const SupportedCNIVersion = "1.0.0"

type NetworkConfig struct {
	CNIVersion string          `json:"cniVersion"`
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	RawIPAM    json.RawMessage `json:"ipam,omitempty"`
}

func ParseConfig(data []byte) (*NetworkConfig, error) {
	var config NetworkConfig

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid CNI configuration: %w", err)
	}

	if config.CNIVersion == "" {
		return nil, errors.New("missing cniVersion")
	}

	if config.CNIVersion != SupportedCNIVersion {
		return nil, fmt.Errorf(
			"unsupported CNI version %q",
			config.CNIVersion,
		)
	}

	if config.Name == "" {
		return nil, errors.New("missing network name")
	}

	if config.Type == "" {
		return nil, errors.New("missing plugin type")
	}

	return &config, nil
}

type RuntimeEnv struct {
	Command     string
	ContainerID string
	NetNS       string
	IfName      string
	CNIPath     string
	Args        string
}

func ReadRuntimeEnv(getenv func(string) string) RuntimeEnv {
	return RuntimeEnv{
		Command:     getenv("CNI_COMMAND"),
		ContainerID: getenv("CNI_CONTAINERID"),
		NetNS:       getenv("CNI_NETNS"),
		IfName:       getenv("CNI_IFNAME"),
		CNIPath:      getenv("CNI_PATH"),
		Args:         getenv("CNI_ARGS"),
	}
}

func (env RuntimeEnv) ValidateForAdd() error {
	if env.Command == "" {
		return errors.New("missing CNI_COMMAND")
	}

	if env.ContainerID == "" {
		return errors.New("missing CNI_CONTAINERID")
	}

	if env.NetNS == "" {
		return errors.New("missing CNI_NETNS")
	}

	if env.IfName == "" {
		return errors.New("missing CNI_IFNAME")
	}

	return nil
}

func (env RuntimeEnv) ValidateForDel() error {
	if env.Command == "" {
		return errors.New("missing CNI_COMMAND")
	}

	if env.ContainerID == "" {
		return errors.New("missing CNI_CONTAINERID")
	}

	if env.IfName == "" {
		return errors.New("missing CNI_IFNAME")
	}

	// CNI_NETNS is optional for DEL.
	return nil
}