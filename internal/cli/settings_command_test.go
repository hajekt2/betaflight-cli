package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestSettingsApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"settings", "set", "gyro_lpf1_static_hz", "0", "--apply", "--auto-port", "--yes"}, nil)
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

func TestSettingsApplyRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"settings", "set", "gyro_lpf1_static_hz", "0", "--apply"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after settings apply confirmation failure")
	}
}

func TestSettingsSetJSONPlanDoesNotConnect(t *testing.T) {
	input := `{"settings":{"small_angle":25,"dshot_bidir":true}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"settings", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if len(lines) != 2 || lines[0] != "set dshot_bidir = ON" || lines[1] != "set small_angle = 25" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	settings := data["settings"].([]any)
	if len(settings) != 2 || settings[0].(map[string]any)["name"] != "dshot_bidir" {
		t.Fatalf("settings = %+v", settings)
	}
	if called {
		t.Fatal("connector was called for plan-only settings set-json command")
	}
}

func TestSettingsSetJSONValidationBeforeConnect(t *testing.T) {
	input := `[{"name":"small_angle","value":181}]`
	env, err := runTestCommandWithInput(t, []string{"settings", "set-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid setting JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSettingsSetJSONApplyWithFakeFC(t *testing.T) {
	input := `[{"name":"small_angle","value":25},{"name":"dshot_bidir","value":true}]`
	env, err := runTestCommandWithInput(t, []string{"settings", "set-json", "-", "--apply", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v: %+v", err, env)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["applied"] != true {
		t.Fatalf("data = %+v", data)
	}
	if len(env.SideEffects) != 2 || env.SideEffects[0].Command != "set small_angle = 25" || env.SideEffects[1].Command != "set dshot_bidir = ON" {
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

func TestSettingsGetIncludesMetadataAndLines(t *testing.T) {
	env, err := runTestCommand(t, []string{"settings", "get", "gyro_lpf1_static_hz"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["setting"] != "gyro_lpf1_static_hz" {
		t.Fatalf("setting = %+v", data["setting"])
	}
	lines := data["lines"].([]any)
	if len(lines) != 1 || lines[0] != "gyro_lpf1_static_hz = 0" {
		t.Fatalf("lines = %+v", lines)
	}
	metadata := data["metadata"].(map[string]any)
	if metadata["name"] != "gyro_lpf1_static_hz" {
		t.Fatalf("metadata = %+v", metadata["name"])
	}
}

func TestSettingsFirmwareGetWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"settings", "firmware-get", "gyro_lpf1_static_hz"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	setting := data["firmware_setting"].(map[string]any)
	if setting["supported"] != true || setting["name"] != "gyro_lpf1_static_hz" || setting["value"] != "42" || setting["msp_name"] != "MSP2_CLI_SETTING" || setting["write_scope"] != "read_only_request" {
		t.Fatalf("firmware_setting = %+v", setting)
	}
	metadata := data["compiled_metadata"].(map[string]any)
	if metadata["name"] != "gyro_lpf1_static_hz" {
		t.Fatalf("compiled_metadata = %+v", metadata)
	}
}

func TestSettingsFirmwareGetReportsUnsupportedTarget(t *testing.T) {
	env, err := runTestCommand(t, []string{"settings", "firmware-get", "gyro_lpf1_static_hz"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		fc := fakefc.New()
		fc.Unsupported[msp.MSP2CLISetting] = true
		client, clientErr := connection.NewClient(fc, time.Second)
		return client, connection.TargetInfo{}, clientErr
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	setting := data["firmware_setting"].(map[string]any)
	if setting["supported"] != false || setting["unsupported_reason"] == "" || setting["name"] != "gyro_lpf1_static_hz" {
		t.Fatalf("firmware_setting = %+v", setting)
	}
	if len(env.Warnings) != 1 || env.Warnings[0].Code != "unsupported_msp" {
		t.Fatalf("warnings = %+v", env.Warnings)
	}
}

func TestSettingsFirmwareInfoWithOffset(t *testing.T) {
	env, err := runTestCommand(t, []string{"settings", "firmware-info", "gyro_lpf1_static_hz", "--offset", "6"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	info := data["firmware_setting_info"].(map[string]any)
	if info["supported"] != true || info["name"] != "gyro_lpf1_static_hz" || info["offset"] != float64(6) || info["msp_name"] != "MSP2_CLI_SETTING_INFO" {
		t.Fatalf("firmware_setting_info identity = %+v", info)
	}
	if info["total_bytes"].(float64) <= info["chunk_bytes"].(float64) || info["complete"] != true {
		t.Fatalf("firmware_setting_info chunk = %+v", info)
	}
	if !strings.Contains(info["text"].(string), "gyro_lpf1_static_hz") || !strings.Contains(info["text"].(string), "type: uint16") {
		t.Fatalf("firmware_setting_info text = %q", info["text"])
	}
}

func TestSettingsFirmwareInfoReportsUnsupportedTarget(t *testing.T) {
	env, err := runTestCommand(t, []string{"settings", "firmware-info", "gyro_lpf1_static_hz"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		fc := fakefc.New()
		fc.Unsupported[msp.MSP2CLISettingInfo] = true
		client, clientErr := connection.NewClient(fc, time.Second)
		return client, connection.TargetInfo{}, clientErr
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	info := data["firmware_setting_info"].(map[string]any)
	if info["supported"] != false || info["unsupported_reason"] == "" || info["name"] != "gyro_lpf1_static_hz" {
		t.Fatalf("firmware_setting_info = %+v", info)
	}
	if len(env.Warnings) != 1 || env.Warnings[0].Code != "unsupported_msp" {
		t.Fatalf("warnings = %+v", env.Warnings)
	}
}

func TestSettingsDiffIncludesConfiguration(t *testing.T) {
	env, err := runTestCommand(t, []string{"settings", "diff"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["command"] != "diff all" {
		t.Fatalf("command = %v", data["command"])
	}
	configuration := data["configuration"].(map[string]any)
	if len(configuration["settings"].([]any)) != 1 {
		t.Fatalf("configuration = %+v", configuration)
	}
	inventory := data["inventory"].(map[string]any)
	if inventory["settings"].(float64) != 1 {
		t.Fatalf("inventory = %+v", inventory)
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
