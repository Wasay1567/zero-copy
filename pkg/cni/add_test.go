package cni

import (
	"context"
	"errors"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
)

type mockNetworkBackend struct {
	addCalled int
	delCalled int

	addResult *Result
	addErr    error
	delErr    error
}

func (m *mockNetworkBackend) Add(
	ctx context.Context,
	env *skel.CmdArgs,
	config *NetworkConfig,
) (*Result, error) {

	m.addCalled++

	return m.addResult, m.addErr
}

func (m *mockNetworkBackend) Del(
	ctx context.Context,
	env *skel.CmdArgs,
	config *NetworkConfig,
) error {

	m.delCalled++

	return m.delErr
}

func validAddEnv() *skel.CmdArgs {
	return &skel.CmdArgs{
		StdinData:   []byte(`{"cniVersion":"1.0.0","name":"zero-copy","type":"zero-copy"}`),
		ContainerID: "container-123",
		Netns:       "/run/netns/container-123",
		IfName:      "eth0",
	}
}

func TestAddError(t *testing.T) {
	expectedErr := errors.New("network setup failed")

	backend := &mockNetworkBackend{
		addErr: expectedErr,
	}

	handler := NewHandler(backend)

	err := handler.CmdAdd(validAddEnv())

	if err == nil {
		t.Fatal("expected ADD error")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected wrapped error, got %v",
			err,
		)
	}
}
