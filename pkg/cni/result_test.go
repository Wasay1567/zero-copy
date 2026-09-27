package cni

import (
	"encoding/json"
	"testing"
)

func TestNewResult(t *testing.T) {
	result := NewResult("1.0.0")

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.CNIVersion != "1.0.0" {
		t.Fatalf(
			"expected 1.0.0, got %s",
			result.CNIVersion,
		)
	}
}

func TestResultGeneration(t *testing.T) {
	result := NewResult("1.0.0")

	result.Interfaces = []*Interface{
		{
			Name:    "eth0",
			Sandbox: "/run/netns/container-123",
		},
	}

	data, err := json.Marshal(result)

	if err != nil {
		t.Fatalf(
			"failed to marshal result: %v",
			err,
		)
	}

	var decoded Result

	err = json.Unmarshal(data, &decoded)

	if err != nil {
		t.Fatalf(
			"failed to decode result: %v",
			err,
		)
	}

	if decoded.CNIVersion != "1.0.0" {
		t.Fatalf(
			"expected 1.0.0, got %s",
			decoded.CNIVersion,
		)
	}

	if len(decoded.Interfaces) != 1 {
		t.Fatalf(
			"expected one interface, got %d",
			len(decoded.Interfaces),
		)
	}

	if decoded.Interfaces[0].Name != "eth0" {
		t.Fatalf(
			"expected eth0, got %s",
			decoded.Interfaces[0].Name,
		)
	}
}
