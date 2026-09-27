package cni

import (
	"context"
	"errors"
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
)

var ErrNotFound = errors.New("network state not found")

// NetworkBackend defines the lifecycle contract for CNI network operations.
// Del must tolerate an absent namespace and return ErrNotFound for absent state.
// State must survive separate plugin invocations; do not rely on in-memory state.
type NetworkBackend interface {
	Add(ctx context.Context, env *skel.CmdArgs, config *NetworkConfig) (*Result, error)
	Del(ctx context.Context, env *skel.CmdArgs, config *NetworkConfig) error
}

type Handler struct {
	Network NetworkBackend
}

func NewHandler(network NetworkBackend) *Handler {
	return &Handler{
		Network: network,
	}
}

// Funcs adapts the testable orchestration to skel's command callbacks.
// skel reads stdin/environment, validates runtime arguments and dispatches commands.
func (h *Handler) Funcs() skel.CNIFuncs {
	return skel.CNIFuncs{
		Add: h.CmdAdd,
		Del: h.CmdDel,
	}
}

func (h *Handler) CmdAdd(args *skel.CmdArgs) error {
	if args == nil {
		return errors.New("missing CNI arguments")
	}
	config, err := ParseConfig(args.StdinData)
	if err != nil {
		return err
	}
	if h.Network == nil {
		return errors.New("network backend is not configured")
	}
	result, err := h.Network.Add(context.Background(), args, config)
	if err != nil {
		return fmt.Errorf("ADD failed: %w", err)
	}
	if result == nil {
		return errors.New("network backend returned nil result")
	}
	if result.CNIVersion == "" {
		result.CNIVersion = config.CNIVersion
	}
	return types.PrintResult(result, config.CNIVersion)
}

func (h *Handler) CmdDel(args *skel.CmdArgs) error {
	if args == nil {
		return errors.New("missing CNI arguments")
	}
	config, err := ParseConfig(args.StdinData)
	if err != nil {
		return err
	}
	if h.Network == nil {
		return errors.New("network backend is not configured")
	}
	err = h.Network.Del(context.Background(), args, config)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("DEL failed: %w", err)
	}
	return nil
}
