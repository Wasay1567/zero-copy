package cni

import (
	"context"
	"errors"
	"testing"
)

func validDelEnv() RuntimeEnv {
	return RuntimeEnv{
		Command:     "DEL",
		ContainerID: "container-123",
		IfName:      "eth0",
	}
}

func TestDelSuccess(t *testing.T) {
	backend := &mockNetworkBackend{}

	handler := NewHandler(backend)

	err := handler.Del(
		context.Background(),
		validDelEnv(),
		validConfig(),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if backend.delCalled != 1 {
		t.Fatalf(
			"expected Del to be called once, got %d",
			backend.delCalled,
		)
	}
}

func TestDelIdempotent(t *testing.T) {
	backend := &mockNetworkBackend{
		delErr: ErrNotFound,
	}

	handler := NewHandler(backend)

	env := validDelEnv()

	// First DEL.
	err := handler.Del(
		context.Background(),
		env,
		validConfig(),
	)

	if err != nil {
		t.Fatalf(
			"first DEL should succeed, got %v",
			err,
		)
	}

	// Second DEL.
	err = handler.Del(
		context.Background(),
		env,
		validConfig(),
	)

	if err != nil {
		t.Fatalf(
			"second DEL should succeed, got %v",
			err,
		)
	}

	if backend.delCalled != 2 {
		t.Fatalf(
			"expected Del to be called twice, got %d",
			backend.delCalled,
		)
	}
}

func TestDelError(t *testing.T) {
	expectedErr := errors.New("network deletion failed")

	backend := &mockNetworkBackend{
		delErr: expectedErr,
	}

	handler := NewHandler(backend)

	err := handler.Del(
		context.Background(),
		validDelEnv(),
		validConfig(),
	)

	if err == nil {
		t.Fatal("expected DEL error")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected wrapped error, got %v",
			err,
		)
	}
}

func TestDelDoesNotRequireNetNS(t *testing.T) {
	backend := &mockNetworkBackend{}

	handler := NewHandler(backend)

	env := validDelEnv()
	env.NetNS = ""

	err := handler.Del(
		context.Background(),
		env,
		validConfig(),
	)

	if err != nil {
		t.Fatalf(
			"DEL should work without NetNS: %v",
			err,
		)
	}
}