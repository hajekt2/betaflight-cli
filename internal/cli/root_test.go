package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func TestClassifyCLI(t *testing.T) {
	tests := []struct {
		command string
		want    cliClass
	}{
		{"diff all", cliReadOnly},
		{"dump all", cliReadOnly},
		{"get gyro_lpf1_static_hz", cliReadOnly},
		{"set gyro_lpf1_static_hz = 0", cliWrite},
		{"save", cliDangerous},
		{"defaults", cliDangerous},
		{"motor 0 1000", cliDangerous},
	}
	for _, tt := range tests {
		if got := classifyCLI(tt.command); got != tt.want {
			t.Fatalf("classifyCLI(%q) = %v, want %v", tt.command, got, tt.want)
		}
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

func TestTelemetrySnapshotWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"telemetry", "snapshot"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data, ok := env.Data.(map[string]any)
	if !ok || data["attitude"] == nil || data["battery"] == nil || data["rc"] == nil {
		t.Fatalf("unexpected telemetry data: %+v", env.Data)
	}
}

func TestSettingsApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"settings", "set", "gyro_lpf1_static_hz", "0", "--apply", "--auto-port"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "cli_command" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSettingsListMetadata(t *testing.T) {
	env, err := runTestCommand(t, []string{"settings", "metadata", "dshot_bidir"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["name"] != "dshot_bidir" {
		t.Fatalf("metadata = %+v", data)
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
	if len(settings) != 2 {
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

func TestFeaturesListWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "list"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	view := data["view"].(map[string]any)
	features := view["features"].([]any)
	if len(features) != 1 {
		t.Fatalf("features = %+v", features)
	}
}

func TestFeatureEnablePlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"features", "enable", "gps"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	lines := data["cli_lines"].([]any)
	if lines[0] != "feature GPS" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only feature command")
	}
}

func TestFeatureEnableApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "enable", "gps", "--apply"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["applied"] != true {
		t.Fatalf("data = %+v", data)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "feature GPS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestModeSetValidationFailureDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"modes", "set", "x", "0", "0", "1700", "2100"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after validation failure")
	}
}

func TestSerialSetPlan(t *testing.T) {
	env, err := runTestCommand(t, []string{"serial", "set", "UART1", "64", "115200", "57600", "0", "115200"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	lines := data["cli_lines"].([]any)
	if lines[0] != "serial UART1 64 115200 57600 0 115200" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
}

func TestRateprofilesListWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"rateprofiles", "list"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	view := data["view"].(map[string]any)
	rateprofiles := view["rateprofiles"].([]any)
	if len(rateprofiles) != 1 {
		t.Fatalf("rateprofiles = %+v", rateprofiles)
	}
}

func TestDomainSaveRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"features", "enable", "gps", "--apply", "--save"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after confirmation failure")
	}
}

func TestSettingsSetValidationFailureDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"settings", "set", "small_angle", "181", "--apply"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after validation failure")
	}
}

func TestSaveRefusalDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"save"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	var ee exitError
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if _, ok := err.(exitError); !ok {
		t.Fatalf("command error = %T %v, want exitError", err, err)
	}
	_ = ee
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called for refused save")
	}
}

func runTestCommand(t *testing.T, args []string, connect connectFunc) (output.Envelope, error) {
	t.Helper()
	var buf bytes.Buffer
	if connect == nil {
		connect = func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
			client, err := connection.NewClient(fakefc.New(), time.Second)
			if err != nil {
				return nil, connection.TargetInfo{}, err
			}
			target, err := client.Handshake(context.Background())
			if err != nil {
				return nil, connection.TargetInfo{}, err
			}
			target.Port = "fake"
			return client, target, nil
		}
	}
	a := &app{
		build:   BuildInfo{Version: "test", Commit: "test", Date: "test"},
		out:     &buf,
		connect: connect,
	}
	root := a.rootCommand()
	root.SetArgs(args)
	err := root.Execute()
	var env output.Envelope
	if decodeErr := json.Unmarshal(buf.Bytes(), &env); decodeErr != nil {
		t.Fatalf("invalid JSON %q: %v", buf.String(), decodeErr)
	}
	return env, err
}
