package cni

import (
	"context"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("network state not found")

func (h *Handler) Del(
	ctx context.Context,
	env RuntimeEnv,
	config *NetworkConfig,
) error {

	if err := env.ValidateForDel(); err != nil {
		return err
	}

	if h.Network == nil {
		return errors.New("network backend is not configured")
	}

	err := h.Network.Del(
		ctx,
		env,
		config,
	)

	if errors.Is(err, ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("DEL failed: %w", err)
	}

	return nil
}