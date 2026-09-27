package cni

import "testing"

func TestParseConfigValid(t *testing.T) {
	data := []byte(`{
		"cniVersion": "1.0.0",
		"name": "zero-copy",
		"type": "zero-copy"
	}`)

	config, err := ParseConfig(data)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if config.CNIVersion != "1.0.0" {
		t.Fatalf(
			"expected version 1.0.0, got %s",
			config.CNIVersion,
		)
	}

	if config.Name != "zero-copy" {
		t.Fatalf(
			"expected name zero-copy, got %s",
			config.Name,
		)
	}

	if config.Type != "zero-copy" {
		t.Fatalf(
			"expected type zero-copy, got %s",
			config.Type,
		)
	}
}

func TestParseConfigInvalidJSON(t *testing.T) {
	data := []byte(`{
		"cniVersion": "1.0.0",
		"name":
	}`)

	_, err := ParseConfig(data)

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseConfigMissingVersion(t *testing.T) {
	data := []byte(`{
		"name": "zero-copy",
		"type": "zero-copy"
	}`)

	_, err := ParseConfig(data)

	if err == nil {
		t.Fatal("expected error for missing cniVersion")
	}
}

func TestParseConfigInvalidVersion(t *testing.T) {
	data := []byte(`{
		"cniVersion": "9.9.9",
		"name": "zero-copy",
		"type": "zero-copy"
	}`)

	_, err := ParseConfig(data)

	if err == nil {
		t.Fatal("expected error for invalid CNI version")
	}
}

func TestParseConfigMissingName(t *testing.T) {
	data := []byte(`{
		"cniVersion": "1.0.0",
		"type": "zero-copy"
	}`)

	_, err := ParseConfig(data)

	if err == nil {
		t.Fatal("expected error for missing network name")
	}
}

func TestParseConfigMissingType(t *testing.T) {
	data := []byte(`{
		"cniVersion": "1.0.0",
		"name": "zero-copy"
	}`)

	_, err := ParseConfig(data)

	if err == nil {
		t.Fatal("expected error for missing plugin type")
	}
}
