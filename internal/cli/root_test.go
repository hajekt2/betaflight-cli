package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
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

func TestBatchPlanFromStdinDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"batch", "plan"}, "feature GPS\nset gyro_lpf1_static_hz = 0\n", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if len(lines) != 2 || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for batch plan")
	}
}

func TestBatchPlanRejectsDangerousLine(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"batch", "plan"}, "save\n", nil)
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "dangerous_action_blocked" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestBatchPlanValidatesSetMetadata(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"batch", "plan"}, "set small_angle = 181\n", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after batch validation failure")
	}
}

func TestBatchApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"batch", "apply"}, "feature GPS\nset gyro_lpf1_static_hz = 0\n", nil)
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
	if len(env.SideEffects) != 2 {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRestorePlanSkipsBackupWrappersDoesNotConnect(t *testing.T) {
	called := false
	input := "# version\nbatch start\ndefaults nosave\nfeature GPS\nset gyro_lpf1_static_hz = 0\nsave\nbatch end\n"
	env, err := runTestCommandWithInput(t, []string{"restore", "plan"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if len(lines) != 2 || lines[0] != "feature GPS" || lines[1] != "set gyro_lpf1_static_hz = 0" {
		t.Fatalf("plan = %+v", data)
	}
	skipped := data["skipped_lines"].([]any)
	if len(skipped) != 5 {
		t.Fatalf("skipped = %+v", skipped)
	}
	if called {
		t.Fatal("connector was called for restore plan")
	}
}

func TestRestorePlanCanIncludeDefaultsNoSave(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"restore", "plan", "--include-defaults"}, "defaults nosave\nfeature GPS\n", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	lines := data["cli_lines"].([]any)
	if len(lines) != 2 || lines[0] != "defaults nosave" || data["include_defaults"] != true {
		t.Fatalf("plan = %+v", data)
	}
}

func TestRestoreApplyIncludeDefaultsRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"restore", "apply", "--include-defaults"}, "defaults nosave\nfeature GPS\n", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after restore confirmation failure")
	}
}

func TestRestoreApplyIncludeDefaultsUsesDangerousOperation(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommandWithInput(t, []string{"restore", "apply", "--include-defaults", "--yes"}, "defaults nosave\nfeature GPS\n", func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		gotOp = op
		return nil, connection.TargetInfo{}, &connection.CodedError{Code: "test_stop", Message: "stop before hardware"}
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "test_stop" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if gotOp != connection.Dangerous {
		t.Fatalf("operation = %v, want Dangerous", gotOp)
	}
}

func TestRestoreApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"restore", "apply"}, "feature GPS\nset gyro_lpf1_static_hz = 0\n", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["applied"] != true || data["saved"] != false {
		t.Fatalf("data = %+v", data)
	}
	if len(env.SideEffects) != 2 {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestPresetsPlanUsesPresetKind(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"presets", "plan"}, "feature GPS\nset small_angle = 25\n", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["kind"] != "preset" || data["source_format"] != "preset_text" {
		t.Fatalf("data = %+v", data)
	}
}

func TestTableDomainListsWithFakeFC(t *testing.T) {
	tests := []struct {
		args []string
		key  string
	}{
		{[]string{"vtxtable", "list"}, "vtx_table"},
		{[]string{"leds", "list"}, "leds"},
		{[]string{"servos", "list"}, "servos"},
		{[]string{"adjustments", "list"}, "adjranges"},
		{[]string{"rxrange", "list"}, "rxranges"},
	}
	for _, tt := range tests {
		env, err := runTestCommand(t, tt.args, nil)
		if err != nil {
			t.Fatalf("%v command error = %v", tt.args, err)
		}
		if !env.OK {
			t.Fatalf("%v env.OK = false: %+v", tt.args, env.Errors)
		}
		data := env.Data.(map[string]any)
		view := data["view"].(map[string]any)
		items := view[tt.key].([]any)
		if len(items) == 0 {
			t.Fatalf("%v view = %+v", tt.args, view)
		}
	}
}

func TestTableDomainSetPlansDoNotConnect(t *testing.T) {
	tests := []struct {
		args []string
		line string
	}{
		{[]string{"vtxtable", "set", "bands", "1"}, "vtxtable bands 1"},
		{[]string{"leds", "set", "0", "0,0::C:0"}, "led 0 0,0::C:0"},
		{[]string{"servos", "set", "0", "1000", "2000", "1500", "100", "none"}, "servo 0 1000 2000 1500 100 -1"},
		{[]string{"servos", "reverse", "0", "2", "r"}, "smix reverse 0 2 r"},
		{[]string{"adjustments", "set", "0", "0", "0", "900", "1300", "12", "0", "0", "0"}, "adjrange 0 0 0 900 1300 12 0 0 0"},
		{[]string{"rxrange", "set", "0", "1000", "2000"}, "rxrange 0 1000 2000"},
	}
	for _, tt := range tests {
		called := false
		env, err := runTestCommand(t, tt.args, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
			called = true
			return nil, connection.TargetInfo{}, nil
		})
		if err != nil {
			t.Fatalf("%v command error = %v", tt.args, err)
		}
		if !env.OK {
			t.Fatalf("%v env.OK = false: %+v", tt.args, env.Errors)
		}
		data := env.Data.(map[string]any)
		lines := data["cli_lines"].([]any)
		if lines[0] != tt.line || data["applied"] != false {
			t.Fatalf("%v plan = %+v", tt.args, data)
		}
		if called {
			t.Fatalf("%v connector was called for plan-only table command", tt.args)
		}
	}
}

func TestTableDomainSetValidationFailureDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"servos", "set", "0", "1000", "2000", "middle", "100", "none"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after table validation failure")
	}
}

func TestTableDomainSetApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"rxrange", "set", "0", "1000", "2000", "--apply"}, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "rxrange 0 1000 2000" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBatchPlanAllowsTableRows(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"batch", "plan"}, "led 0 0,0::C:0\nservo 0 1000 2000 1500 100 -1\nadjrange 0 0 0 900 1300 12 0 0 0\nrxrange 0 1000 2000\n", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	lines := data["cli_lines"].([]any)
	if len(lines) != 4 {
		t.Fatalf("plan = %+v", data)
	}
}

func TestSettingDomainListsWithFakeFC(t *testing.T) {
	tests := []struct {
		args []string
		key  string
	}{
		{[]string{"pid", "list"}, "settings"},
		{[]string{"rates", "list"}, "settings"},
		{[]string{"filters", "list"}, "settings"},
		{[]string{"receiver", "list"}, "settings"},
		{[]string{"vtx", "list"}, "settings"},
		{[]string{"osd", "list"}, "settings"},
		{[]string{"gps", "list"}, "settings"},
		{[]string{"failsafe", "list"}, "settings"},
	}
	for _, tt := range tests {
		env, err := runTestCommand(t, tt.args, nil)
		if err != nil {
			t.Fatalf("%v command error = %v", tt.args, err)
		}
		if !env.OK {
			t.Fatalf("%v env.OK = false: %+v", tt.args, env.Errors)
		}
		data := env.Data.(map[string]any)
		view := data["view"].(map[string]any)
		items := view[tt.key].([]any)
		if len(items) == 0 {
			t.Fatalf("%v view = %+v", tt.args, view)
		}
	}
}

func TestSettingDomainSetPlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"pid", "set", "p_roll", "46"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if lines[0] != "set p_roll = 46" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only domain set")
	}
}

func TestSettingDomainSetRejectsWrongDomain(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"pid", "set", "roll_rc_rate", "8"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "unknown_setting" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after domain validation failure")
	}
}

func TestSettingDomainSetApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"pid", "set", "p_roll", "46", "--apply"}, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "set p_roll = 46" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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
	return runTestCommandWithInput(t, args, "", connect)
}

func runTestCommandWithInput(t *testing.T, args []string, input string, connect connectFunc) (output.Envelope, error) {
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
		in:      strings.NewReader(input),
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
