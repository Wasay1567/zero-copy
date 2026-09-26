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

func TestReadRuntimeEnv(t *testing.T) {
	values := map[string]string{
		"CNI_COMMAND":     "ADD",
		"CNI_CONTAINERID": "container-123",
		"CNI_NETNS":       "/run/netns/container-123",
		"CNI_IFNAME":      "eth0",
		"CNI_PATH":        "/opt/cni/bin",
		"CNI_ARGS":        "K8S_POD_NAME=test",
	}

	getenv := func(key string) string {
		return values[key]
	}

	env := ReadRuntimeEnv(getenv)

	if env.Command != "ADD" {
		t.Errorf("expected ADD, got %s", env.Command)
	}

	if env.ContainerID != "container-123" {
		t.Errorf(
			"expected container-123, got %s",
			env.ContainerID,
		)
	}

	if env.NetNS != "/run/netns/container-123" {
		t.Errorf(
			"unexpected network namespace: %s",
			env.NetNS,
		)
	}

	if env.IfName != "eth0" {
		t.Errorf(
			"expected eth0, got %s",
			env.IfName,
		)
	}
}

func TestValidateAddMissingContainerID(t *testing.T) {
	env := RuntimeEnv{
		Command: "ADD",
		NetNS:   "/run/netns/test",
		IfName:  "eth0",
	}

	err := env.ValidateForAdd()

	if err == nil {
		t.Fatal("expected error for missing container ID")
	}
}

func TestValidateAddMissingNetNS(t *testing.T) {
	env := RuntimeEnv{
		Command:     "ADD",
		ContainerID: "container-123",
		IfName:      "eth0",
	}

	err := env.ValidateForAdd()

	if err == nil {
		t.Fatal("expected error for missing network namespace")
	}
}

func TestValidateAddMissingIfName(t *testing.T) {
	env := RuntimeEnv{
		Command:     "ADD",
		ContainerID: "container-123",
		NetNS:       "/run/netns/test",
	}

	err := env.ValidateForAdd()

	if err == nil {
		t.Fatal("expected error for missing interface name")
	}
}

func TestValidateDelDoesNotRequireNetNS(t *testing.T) {
	env := RuntimeEnv{
		Command:     "DEL",
		ContainerID: "container-123",
		IfName:      "eth0",
	}

	err := env.ValidateForDel()

	if err != nil {
		t.Fatalf(
			"DEL should not require CNI_NETNS: %v",
			err,
		)
	}
}