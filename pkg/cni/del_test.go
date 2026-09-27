package cni

import (
	"errors"
	"fmt"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
)

func validDelEnv() *skel.CmdArgs {
	return &skel.CmdArgs{
		StdinData:   []byte(`{"cniVersion":"1.0.0","name":"zero-copy","type":"zero-copy"}`),
		ContainerID: "container-123",
		IfName:      "eth0",
	}
}

func TestDelSuccess(t *testing.T) {
	backend := &mockNetworkBackend{}

	handler := NewHandler(backend)

	err := handler.CmdDel(validDelEnv())

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
	backend := &mockNetworkBackend{}

	handler := NewHandler(backend)

	env := validDelEnv()

	// First DEL.
	err := handler.CmdDel(env)

	if err != nil {
		t.Fatalf(
			"first DEL should succeed, got %v",
			err,
		)
	}

	// The first call removed the state; subsequent calls report it absent.
	backend.delErr = fmt.Errorf("already deleted: %w", ErrNotFound)

	// Second DEL.
	err = handler.CmdDel(env)

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

	err := handler.CmdDel(validDelEnv())

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

func TestDelDoesNotRequireNetns(t *testing.T) {
	backend := &mockNetworkBackend{}

	handler := NewHandler(backend)

	env := validDelEnv()
	env.Netns = ""

	err := handler.CmdDel(env)

	if err != nil {
		t.Fatalf(
			"DEL should work without Netns: %v",
			err,
		)
	}
}
