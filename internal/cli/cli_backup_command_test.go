package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func TestCLIExecWriteRequiresYes(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"cli", "exec", "set gyro_lpf1_static_hz = 0"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("cli exec was allowed without --yes")
	}
	env, err = runTestCommand(t, []string{"cli", "exec", "set gyro_lpf1_static_hz = 0", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("expected OK with --yes, got errors: %+v", env.Errors)
	}
}

func TestCLIExecUnknownCommandIsWriteClassified(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"cli", "exec", "mystery command"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("cli exec unknown command was allowed without --yes")
	}

	env, err = runTestCommand(t, []string{"cli", "exec", "mystery command", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("expected OK with --yes, got errors: %+v", env.Errors)
	}
}

func TestCLIExecClassifiesCommandSequencesBeforeConnect(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantCode string
	}{
		{"semicolon", "diff all; save", "confirmation_required"},
		{"newline", "get small_angle\nmotor 0 1100", "dangerous_action_blocked"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			env, err := runTestCommand(t, []string{"cli", "exec", tt.command}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
				called = true
				return nil, connection.TargetInfo{}, nil
			})
			if err == nil {
				t.Fatal("command error = nil, want non-zero exit")
			}
			if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != tt.wantCode {
				t.Fatalf("unexpected envelope: %+v", env)
			}
			if called {
				t.Fatal("cli exec sequence was allowed without --yes")
			}
		})
	}
}

func TestCLIExecBlocksRawMotorCommandsEvenWithYes(t *testing.T) {
	for _, command := range []string{"motor 0 1100", "get small_angle; dshotprog 0 1"} {
		t.Run(command, func(t *testing.T) {
			called := false
			env, err := runTestCommand(t, []string{"cli", "exec", command, "--yes"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
				called = true
				return nil, connection.TargetInfo{}, nil
			})
			if err == nil || env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "dangerous_action_blocked" {
				t.Fatalf("unexpected envelope: %+v, err: %v", env, err)
			}
			if called {
				t.Fatal("blocked raw actuation command attempted transport")
			}
		})
	}
}

func TestCLIInteractiveRequiresYes(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"cli", "interactive"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("cli interactive was allowed without --yes")
	}
}

func TestParseCLISections(t *testing.T) {
	sections := parseCLISections([]string{
		"# comment",
		"set gyro_lpf1_static_hz = 0",
		"serial 0 1 115200 57600 0 115200",
		"feature GPS",
		"aux 0 0 0 1700 2100 0 0",
		"resource MOTOR 1 A00",
		"vtxtable bands 5",
		"unknown stuff",
	})
	if len(sections["settings"]) != 1 {
		t.Fatalf("settings = %+v", sections["settings"])
	}
	if len(sections["unknown"]) != 1 {
		t.Fatalf("unknown = %+v", sections["unknown"])
	}
}

func TestRedactLines(t *testing.T) {
	lines, classes := redactLines([]string{
		"set pilot_name = Tomas",
		"set gyro_lpf1_static_hz = 0",
	})
	if lines[0] != "set pilot_name = REDACTED" {
		t.Fatalf("redacted line = %q", lines[0])
	}
	if len(classes) != 1 || classes[0] != "pilot_name" {
		t.Fatalf("classes = %+v", classes)
	}
}

func TestBackupCreateIncludesConfiguration(t *testing.T) {
	env, err := runTestCommand(t, []string{"backup", "create"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	configuration := data["configuration"].(map[string]any)
	settings := configuration["settings"].([]any)
	if len(settings) < 8 {
		t.Fatalf("settings = %+v", settings)
	}
	features := configuration["features"].([]any)
	if len(features) != 1 {
		t.Fatalf("features = %+v", features)
	}
	if data["raw_authoritative"] != true {
		t.Fatalf("raw_authoritative = %+v", data["raw_authoritative"])
	}
}

func TestBackupCreateRawCLI(t *testing.T) {
	env, err := runTestCommand(t, []string{"backup", "create", "--raw-cli"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	raw, ok := env.Data.(string)
	if !ok {
		t.Fatalf("data = %+v", env.Data)
	}
	if !strings.Contains(raw, "batch start") || !strings.Contains(raw, "set gyro_lpf1_static_hz = 0") {
		t.Fatalf("raw = %q", raw)
	}
}

func TestBackupCreateRawCLISupportsRedact(t *testing.T) {
	env, err := runTestCommand(t, []string{"backup", "create", "--raw-cli", "--redact"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	raw := env.Data.(string)
	if !strings.Contains(raw, "batch start") {
		t.Fatalf("raw = %q", raw)
	}
}

func TestCLIExecDiffIncludesConfiguration(t *testing.T) {
	env, err := runTestCommand(t, []string{"cli", "exec", "diff all"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	configuration := data["configuration"].(map[string]any)
	if len(configuration["features"].([]any)) != 1 {
		t.Fatalf("configuration = %+v", configuration)
	}
	if len(configuration["settings"].([]any)) != 1 {
		t.Fatalf("configuration = %+v", configuration)
	}
}

func TestCLIDiffCommandIncludesConfiguration(t *testing.T) {
	env, err := runTestCommand(t, []string{"cli", "diff"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	configuration := data["configuration"].(map[string]any)
	if len(configuration["settings"].([]any)) != 1 {
		t.Fatalf("configuration = %+v", configuration)
	}
	if data["command"] != "diff all" {
		t.Fatalf("command = %v", data["command"])
	}
}

func TestCLIDumpCommandIncludesConfiguration(t *testing.T) {
	env, err := runTestCommand(t, []string{"cli", "dump"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["command"] != "dump all" {
		t.Fatalf("command = %v", data["command"])
	}
	if len(data["lines"].([]any)) == 0 {
		t.Fatalf("lines = %+v", data["lines"])
	}
}
