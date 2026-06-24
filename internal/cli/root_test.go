package cli

import (
	"context"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
)

func TestRootRegistersArchitectureGlobalFlags(t *testing.T) {
	root := (&app{}).rootCommand()
	for _, name := range []string{"port", "auto-port", "allow-unsupported", "baud", "timeout", "format", "verbose", "yes"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("missing persistent flag %q", name)
		}
	}
}

func TestVerboseAddsConnectionDiagnosticsToConnectedCommand(t *testing.T) {
	env, err := runTestCommand(t, []string{"--verbose", "--port", "fake-port", "--baud", "57600", "--timeout", "750ms", "info"}, func(_ context.Context, cfg connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		if op != connection.ReadOnly {
			t.Fatalf("operation = %v, want read-only", op)
		}
		if cfg.Port != "fake-port" || cfg.Baud != 57600 || cfg.Timeout != 750*time.Millisecond {
			t.Fatalf("connection config = %+v", cfg)
		}
		client, clientErr := connection.NewClient(fakefc.New(), time.Second)
		if clientErr != nil {
			return nil, connection.TargetInfo{}, clientErr
		}
		target, targetErr := client.Handshake(context.Background())
		if targetErr != nil {
			return nil, connection.TargetInfo{}, targetErr
		}
		target.Port = cfg.Port
		return client, target, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	diagnostics := env.Data.(map[string]any)["diagnostics"].(map[string]any)
	conn := diagnostics["connection"].(map[string]any)
	if conn["operation"] != "read_only" {
		t.Fatalf("operation = %v", conn["operation"])
	}
	config := conn["config"].(map[string]any)
	if config["port"] != "fake-port" || config["baud"] != float64(57600) || config["timeout"] != "750ms" {
		t.Fatalf("config = %+v", config)
	}
	target := conn["target"].(map[string]any)
	if target["port"] != "fake-port" || target["variant"] != "BTFL" {
		t.Fatalf("target = %+v", target)
	}
	support := conn["support"].(map[string]any)
	if support["supported"] != true {
		t.Fatalf("support = %+v", support)
	}
}

func TestVerboseAddsConnectionDiagnosticsToFailure(t *testing.T) {
	env, err := runTestCommand(t, []string{"--verbose", "info"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		return nil, connection.TargetInfo{}, &connection.CodedError{
			Code:    "no_target",
			Message: "no Betaflight-compatible USB serial port responded to MSP_API_VERSION",
		}
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if !isExitError(err) {
		t.Fatalf("command error = %T %v, want exitError", err, err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "no_target" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	diagnostics := env.Data.(map[string]any)["diagnostics"].(map[string]any)
	conn := diagnostics["connection"].(map[string]any)
	failure := conn["error"].(map[string]any)
	if failure["code"] != "no_target" || failure["candidate_count"] != float64(0) {
		t.Fatalf("failure diagnostics = %+v", failure)
	}
	if diagnostics["port_diagnostics"] == nil {
		t.Fatalf("missing port diagnostics: %+v", diagnostics)
	}
}
