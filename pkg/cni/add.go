package cni

import (
	"context"
	"errors"
	"fmt"
)


func (h *Handler) Add(
	ctx context.Context,
	env RuntimeEnv,
	config *NetworkConfig,
) (*Result, error) {

	if err := env.ValidateForAdd(); err != nil {
		return nil, err
	}

	if h.Network == nil {
		return nil, errors.New("network backend is not configured")
	}

	result, err := h.Network.Add(
		ctx,
		env,
		config,
	)

	if err != nil {
		return nil, fmt.Errorf("ADD failed: %w", err)
	}

	if result == nil {
		return nil, errors.New(
			"network backend returned nil result",
		)
	}

	if result.CNIVersion == "" {
		result.CNIVersion = config.CNIVersion
	}

	return result, nil
}