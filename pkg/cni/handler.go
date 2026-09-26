package cni

import "context"

// NetworkBackend defines the lifecycle contract for CNI network operations.
type NetworkBackend interface {
    Add(ctx context.Context, env RuntimeEnv, config *NetworkConfig) (*Result, error)
    Del(ctx context.Context, env RuntimeEnv, config *NetworkConfig) error
}


type Handler struct {
	Network NetworkBackend
}

func NewHandler(network NetworkBackend) *Handler {
	return &Handler{
		Network: network,
	}
}