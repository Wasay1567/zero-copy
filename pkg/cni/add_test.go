package cni

import (
	"context"
	"errors"
	"testing"
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
	env RuntimeEnv,
	config *NetworkConfig,
) (*Result, error) {

	m.addCalled++

	return m.addResult, m.addErr
}

func (m *mockNetworkBackend) Del(
	ctx context.Context,
	env RuntimeEnv,
	config *NetworkConfig,
) error {

	m.delCalled++

	return m.delErr
}

func validConfig() *NetworkConfig {
	return &NetworkConfig{
		CNIVersion: "1.0.0",
		Name:       "zero-copy",
		Type:       "zero-copy",
	}
}

func validAddEnv() RuntimeEnv {
	return RuntimeEnv{
		Command:     "ADD",
		ContainerID: "container-123",
		NetNS:       "/run/netns/container-123",
		IfName:      "eth0",
	}
}

func TestAddSuccess(t *testing.T) {
	backend := &mockNetworkBackend{
		addResult: NewResult("1.0.0"),
	}

	handler := NewHandler(backend)

	result, err := handler.Add(
		context.Background(),
		validAddEnv(),
		validConfig(),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.CNIVersion != "1.0.0" {
		t.Fatalf(
			"expected 1.0.0, got %s",
			result.CNIVersion,
		)
	}

	if backend.addCalled != 1 {
		t.Fatalf(
			"expected Add to be called once, got %d",
			backend.addCalled,
		)
	}
}

func TestAddError(t *testing.T) {
	expectedErr := errors.New("network setup failed")

	backend := &mockNetworkBackend{
		addErr: expectedErr,
	}

	handler := NewHandler(backend)

	_, err := handler.Add(
		context.Background(),
		validAddEnv(),
		validConfig(),
	)

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

func TestAddMissingContainerID(t *testing.T) {
	backend := &mockNetworkBackend{}

	handler := NewHandler(backend)

	env := validAddEnv()
	env.ContainerID = ""

	_, err := handler.Add(
		context.Background(),
		env,
		validConfig(),
	)

	if err == nil {
		t.Fatal("expected missing container ID error")
	}

	if backend.addCalled != 0 {
		t.Fatal("backend should not be called")
	}
}

func TestAddMissingNetworkNamespace(t *testing.T) {
	backend := &mockNetworkBackend{}

	handler := NewHandler(backend)

	env := validAddEnv()
	env.NetNS = ""

	_, err := handler.Add(
		context.Background(),
		env,
		validConfig(),
	)

	if err == nil {
		t.Fatal("expected missing network namespace error")
	}

	if backend.addCalled != 0 {
		t.Fatal("backend should not be called")
	}
}