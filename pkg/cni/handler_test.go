package cni

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
)

// Run skel in an isolated process because its public API uses process stdin/env.
func TestSkelProcess(t *testing.T) {
	if os.Getenv("ZERO_COPY_SKEL_TEST") != "1" {
		return
	}
	backend := &mockNetworkBackend{addResult: NewResult(SupportedCNIVersion)}
	backend.addResult.Interfaces = []*Interface{{Name: "eth0", Sandbox: "/test/netns"}}
	if os.Getenv("ZERO_COPY_BACKEND_ERROR") == "1" {
		backend.addErr = fmt.Errorf("setup failed")
	}
	backend.delErr = fmt.Errorf("already removed: %w", ErrNotFound)
	h := NewHandler(backend)
	funcs := h.Funcs()
	add := funcs.Add
	funcs.Add = func(args *skel.CmdArgs) error {
		if args.ContainerID != "container-123" || args.Netns != "/test/netns" || args.IfName != "eth0" || args.Path != "/opt/cni/bin" || args.Args != "IgnoreUnknown=1" {
			return fmt.Errorf("runtime arguments were not forwarded: %+v", args)
		}
		return add(args)
	}
	skel.PluginMainFuncs(funcs, version.PluginSupports(SupportedCNIVersion), "")
	os.Exit(0)
}

func TestSkelDispatch(t *testing.T) {
	config := `{"cniVersion":"1.0.0","name":"zero-copy","type":"zero-copy"}`
	tests := []struct {
		name, command, config, missing string
		fail                           bool
		backendError                   bool
	}{
		{name: "ADD", command: "ADD", config: config},
		{name: "DEL without namespace", command: "DEL", config: config, missing: "CNI_NETNS"},
		{name: "repeated DEL", command: "DEL", config: config, missing: "CNI_NETNS"},
		{name: "VERSION", command: "VERSION"},
		{name: "missing container", command: "ADD", config: config, missing: "CNI_CONTAINERID", fail: true},
		{name: "missing namespace", command: "ADD", config: config, missing: "CNI_NETNS", fail: true},
		{name: "missing interface", command: "ADD", config: config, missing: "CNI_IFNAME", fail: true},
		{name: "missing path", command: "ADD", config: config, missing: "CNI_PATH", fail: true},
		{name: "missing command", config: config, fail: true},
		{name: "invalid JSON", command: "ADD", config: `{`, fail: true},
		{name: "missing config", command: "ADD", fail: true},
		{name: "missing type", command: "ADD", config: `{"cniVersion":"1.0.0","name":"test"}`, fail: true},
		{name: "invalid version", command: "ADD", config: `{"cniVersion":"9.9.9","name":"test","type":"zero-copy"}`, fail: true},
		{name: "backend failure", command: "ADD", config: config, fail: true, backendError: true},
		{name: "unknown command", command: "BOGUS", config: config, fail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestSkelProcess$")
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "CNI_") && !strings.HasPrefix(entry, "ZERO_COPY_") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			env := map[string]string{"ZERO_COPY_SKEL_TEST": "1", "CNI_COMMAND": tt.command, "CNI_CONTAINERID": "container-123", "CNI_NETNS": "/test/netns", "CNI_IFNAME": "eth0", "CNI_PATH": "/opt/cni/bin", "CNI_ARGS": "IgnoreUnknown=1", "CNI_NETNS_OVERRIDE": "1"}
			// Only the mock process bypasses skel's real namespace check.
			delete(env, tt.missing)
			if tt.backendError {
				env["ZERO_COPY_BACKEND_ERROR"] = "1"
			}
			for k, v := range env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			cmd.Stdin = strings.NewReader(tt.config)
			output, err := cmd.Output()
			if tt.fail {
				if err == nil {
					t.Fatalf("expected failure, output: %s", output)
				}
				var e types.Error
				if err := json.Unmarshal(output, &e); err != nil || e.Code == 0 {
					t.Fatalf("invalid CNI error: %s (%v)", output, err)
				}
				if tt.missing != "" && (e.Code != types.ErrInvalidEnvironmentVariables || !strings.Contains(e.Msg, tt.missing)) {
					t.Fatalf("expected missing %s environment error, got %+v", tt.missing, e)
				}

				return
			}
			if err != nil {
				t.Fatalf("command failed: %v: %s", err, output)
			}
			switch tt.command {
			case "ADD":
				result, err := version.NewResult(SupportedCNIVersion, output)
				if err != nil {
					t.Fatalf("invalid CNI result: %v", err)
				}
				r, err := result.GetAsVersion(SupportedCNIVersion)
				if err != nil || len(r.(*Result).Interfaces) != 1 {
					t.Fatalf("unexpected result: %s", output)
				}
			case "DEL":
				if len(output) != 0 {
					t.Fatalf("DEL wrote output: %s", output)
				}
			case "VERSION":
				if !strings.Contains(string(output), `"1.0.0"`) {
					t.Fatalf("invalid VERSION: %s", output)
				}
			}
		})
	}
}

func TestHandlerInvalidInputs(t *testing.T) {
	for _, config := range []string{"", "{}", `{"cniVersion":"9.9.9","name":"test","type":"zero-copy"}`} {
		args := validAddEnv()
		args.StdinData = []byte(config)
		backend := &mockNetworkBackend{}
		h := NewHandler(backend)
		if err := h.CmdAdd(args); err == nil {
			t.Fatal("ADD accepted invalid config")
		}
		if err := h.CmdDel(args); err == nil {
			t.Fatal("DEL accepted invalid config")
		}
		if backend.addCalled+backend.delCalled != 0 {
			t.Fatal("invalid config reached backend")
		}
	}
	h := NewHandler(nil)
	if err := h.CmdAdd(validAddEnv()); err == nil {
		t.Fatal("missing backend accepted")
	}
	if err := h.CmdDel(validDelEnv()); err == nil {
		t.Fatal("missing backend accepted")
	}
	h = NewHandler(&mockNetworkBackend{})
	if err := h.CmdAdd(validAddEnv()); err == nil {
		t.Fatal("nil result accepted")
	}
	if err := h.CmdAdd(nil); err == nil {
		t.Fatal("nil arguments accepted")
	}
	if err := h.CmdDel(nil); err == nil {
		t.Fatal("nil arguments accepted")
	}
}
