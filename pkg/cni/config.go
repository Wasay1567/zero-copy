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

	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &config, nil
}

// Validate also protects callers that construct configuration directly.
func (config *NetworkConfig) Validate() error {
	if config == nil {
		return errors.New("missing CNI configuration")
	}
	if config.CNIVersion == "" {
		return errors.New("missing cniVersion")
	}

	if config.CNIVersion != SupportedCNIVersion {
		return fmt.Errorf(
			"unsupported CNI version %q",
			config.CNIVersion,
		)
	}

	if config.Name == "" {
		return errors.New("missing network name")
	}

	if config.Type == "" {
		return errors.New("missing plugin type")
	}

	return nil
}
