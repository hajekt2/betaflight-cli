package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
)

func TestConfigurationStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"configuration", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	configuration := data["configuration"].(map[string]any)
	summary := configuration["summary"].(map[string]any)
	if summary["configuration_state"] != "CONFIGURED" || summary["configured"] != true {
		t.Fatalf("summary = %+v", summary)
	}
	if summary["reboot_required"] != false || summary["arming_blocked"] != true {
		t.Fatalf("runtime summary = %+v", summary)
	}
	if summary["pid_profile"] != float64(0) || summary["battery_profile"] != float64(1) {
		t.Fatalf("profile summary = %+v", summary)
	}
	guidance := configuration["guidance"].(map[string]any)
	if guidance["read_only_snapshot"] != true || guidance["plan_before_apply"] != true || guidance["save_is_explicit"] != true {
		t.Fatalf("guidance = %+v", guidance)
	}
	if configuration["system"] == nil || configuration["runtime"] == nil || configuration["profiles"] == nil {
		t.Fatalf("sections = %+v", configuration)
	}
}

func TestConfigurationSnapshotWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"configuration", "snapshot"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	snapshot := data["configuration_snapshot"].(map[string]any)
	summary := snapshot["summary"].(map[string]any)
	if summary["full_line_count"].(float64) <= summary["diff_line_count"].(float64) {
		t.Fatalf("summary counts = %+v", summary)
	}
	if summary["diff_has_save_command"] != true || summary["review_required"] != true {
		t.Fatalf("summary review = %+v", summary)
	}
	guidance := snapshot["guidance"].(map[string]any)
	if guidance["read_only_snapshot"] != true || guidance["strip_save_before_restore"] != true {
		t.Fatalf("guidance = %+v", guidance)
	}
	diff := snapshot["diff"].(map[string]any)
	if diff["source_command"] != "diff all" || diff["unknown_count"].(float64) != 0 {
		t.Fatalf("diff = %+v", diff)
	}
	sectionCounts := diff["section_counts"].(map[string]any)
	if sectionCounts["batch"].(float64) != 1 || sectionCounts["board"].(float64) != 2 || sectionCounts["defaults"].(float64) != 1 || sectionCounts["save"].(float64) != 1 {
		t.Fatalf("section counts = %+v", sectionCounts)
	}
	full := snapshot["full"].(map[string]any)
	if full["source_command"] != "dump all" || full["line_count"].(float64) == 0 {
		t.Fatalf("full = %+v", full)
	}
}

func TestConfigurationValidateDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"configuration", "validate"}, "# version\nbatch start\nfeature GPS\nset gyro_lpf1_static_hz = 0\nsave\n", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if called {
		t.Fatal("connector was called for configuration validate")
	}
	data := env.Data.(map[string]any)
	validation := data["configuration_validation"].(map[string]any)
	inventory := validation["inventory"].(map[string]any)
	if inventory["settings"] != float64(1) || inventory["features_enabled"].([]any)[0] != "GPS" {
		t.Fatalf("inventory = %+v", inventory)
	}
	result := validation["validation"].(map[string]any)
	if result["valid"] != true || result["review_required"] != false {
		t.Fatalf("validation = %+v", result)
	}
	skipped := validation["skipped_lines"].([]any)
	if len(skipped) != 3 {
		t.Fatalf("skipped = %+v", skipped)
	}
	plan := validation["plan"].(map[string]any)
	lines := plan["cli_lines"].([]any)
	if len(lines) != 2 || lines[0] != "feature GPS" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestConfigurationPlanWithoutConnection(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"configuration", "plan"}, "feature GPS\nset gyro_lpf1_static_hz = 0\n", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)
	if plan["kind"] != "configuration" || plan["source_format"] != "configuration_text" {
		t.Fatalf("plan = %+v", plan)
	}
	lines := plan["cli_lines"].([]any)
	if len(lines) != 2 || lines[0] != "feature GPS" || lines[1] != "set gyro_lpf1_static_hz = 0" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestConfigurationPlanAcceptsJSON(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"configuration", "plan", "--file", "-"}, `{"data":{"lines":["feature GPS","set gyro_lpf1_static_hz = 0"]}}`, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["source_format"] != "json" || data["kind"] != "configuration" {
		t.Fatalf("plan = %+v", data)
	}
	lines := data["cli_lines"].([]any)
	if len(lines) != 2 || lines[0] != "feature GPS" || lines[1] != "set gyro_lpf1_static_hz = 0" {
		t.Fatalf("plan = %+v", data)
	}
}

func TestConfigurationPlanSkipsUnsupportedLines(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"configuration", "plan", "--include-defaults"}, "# version\nbatch start\ndefaults nosave\nfeature GPS\nset gyro_lpf1_static_hz = 0\nsave\nbatch end\n", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)
	lines := plan["cli_lines"].([]any)
	if len(lines) != 3 || lines[0] != "defaults nosave" || lines[1] != "feature GPS" {
		t.Fatalf("plan = %+v", plan)
	}
	skipped := plan["skipped_lines"].([]any)
	if len(skipped) != 4 || skipped[0].(map[string]any)["reason"] == "" {
		t.Fatalf("skipped = %+v", skipped)
	}
}

func TestConfigurationApplyUsesWriteOperation(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommandWithInput(t, []string{"configuration", "apply", "--yes"}, "feature GPS\nset gyro_lpf1_static_hz = 0\n", func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		gotOp = op
		return nil, connection.TargetInfo{}, &connection.CodedError{Code: "test_stop", Message: "stop before hardware"}
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "test_stop" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if gotOp != connection.Write {
		t.Fatalf("operation = %v, want Write", gotOp)
	}
	data := env.Data.(map[string]any)
	if data["kind"] != "configuration" {
		t.Fatalf("plan = %+v", data)
	}
}

func TestConfigurationApplyRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"configuration", "apply"}, "feature GPS\n", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatalf("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after configuration apply confirmation failure")
	}
}

func TestConfigurationApplyIncludeDefaultsRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"configuration", "apply", "--include-defaults"}, "defaults nosave\nfeature GPS\n", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatalf("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after configuration apply confirmation failure")
	}
}

func TestConfigurationApplySaveRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"configuration", "apply", "--save"}, "feature GPS\n", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after configuration apply save confirmation failure")
	}
}

func TestConfigurationApplyWithSaveWritesAndSaves(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommandWithInput(t, []string{"configuration", "apply", "--save", "--yes"}, "feature GPS\n", func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		gotOp = op
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
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if gotOp != connection.Dangerous {
		t.Fatalf("operation = %v, want Dangerous", gotOp)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["applied"] != true || data["saved"] != true {
		t.Fatalf("data = %+v", data)
	}
	if len(env.SideEffects) != 2 {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
	if env.SideEffects[0].Type != "cli_command" || env.SideEffects[1].Type != "save" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestConfigurationApplyWithJSONFileCanSave(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommandWithInput(t, []string{"configuration", "apply", "--file", "-", "--save", "--yes"}, `{"data":{"lines":["feature GPS"]}}`, func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		gotOp = op
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
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if gotOp != connection.Dangerous {
		t.Fatalf("operation = %v, want Dangerous", gotOp)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["source_format"] != "json" || data["kind"] != "configuration" || data["applied"] != true || data["saved"] != true {
		t.Fatalf("plan = %+v", data)
	}
}

func TestConfigurationValidateReportsDangerousLine(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"configuration", "validate"}, "reboot\nfeature GPS\n", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	validation := data["configuration_validation"].(map[string]any)["validation"].(map[string]any)
	if validation["valid"] != false {
		t.Fatalf("validation = %+v", validation)
	}
	errors := validation["errors"].([]any)
	if len(errors) != 1 || errors[0].(map[string]any)["code"] != "dangerous_action_blocked" {
		t.Fatalf("errors = %+v", errors)
	}
}

func TestConfigurationValidateAcceptsJSONLines(t *testing.T) {
	input := `{"data":{"lines":["# version","batch start","feature GPS","set gyro_lpf1_static_hz = 0","save"]}}`
	env, err := runTestCommandWithInput(t, []string{"configuration", "validate"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	validation := env.Data.(map[string]any)["configuration_validation"].(map[string]any)
	if validation["source_format"] != "json" {
		t.Fatalf("validation = %+v", validation)
	}
	plan := validation["plan"].(map[string]any)
	lines := plan["cli_lines"].([]any)
	if len(lines) != 2 || lines[1] != "set gyro_lpf1_static_hz = 0" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestConfigurationValidateAcceptsFullDumpFamilies(t *testing.T) {
	input := strings.Join([]string{
		"# version",
		"batch start",
		"board_name FAKEF405",
		"manufacturer_id FAKE",
		"timer A00 AF1",
		"dma pin A00 1",
		"mixer QUADX",
		"mmix reset",
		"map AETR1234",
		"beeper -BAT_LOW",
		"beacon RX_SET",
		"save",
	}, "\n")
	env, err := runTestCommandWithInput(t, []string{"configuration", "validate"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	validation := env.Data.(map[string]any)["configuration_validation"].(map[string]any)
	result := validation["validation"].(map[string]any)
	if result["valid"] != true {
		t.Fatalf("validation = %+v", result)
	}
	skipped := validation["skipped_lines"].([]any)
	if len(skipped) != 5 {
		t.Fatalf("skipped = %+v", skipped)
	}
}

func TestConfigurationCompareWithFakeFC(t *testing.T) {
	reference := "# version\nbatch start\nfeature GPS\nset gyro_lpf1_static_hz = 0\nset p_roll = 46\nsave\n"
	env, err := runTestCommandWithInput(t, []string{"configuration", "compare"}, reference, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	compare := data["configuration_compare"].(map[string]any)
	summary := compare["summary"].(map[string]any)
	if summary["matches"] != false || summary["changed_setting_count"] != float64(1) {
		t.Fatalf("summary = %+v", summary)
	}
	settingDiff := compare["setting_diff"].(map[string]any)
	changed := settingDiff["changed"].([]any)
	if len(changed) != 1 || changed[0].(map[string]any)["name"] != "p_roll" {
		t.Fatalf("changed = %+v", changed)
	}
	lineDiff := compare["line_diff"].(map[string]any)
	if len(lineDiff["only_in_current"].([]any)) == 0 {
		t.Fatalf("line diff = %+v", lineDiff)
	}
}

func TestConfigurationCompareAcceptsJSONRaw(t *testing.T) {
	reference := `{"data":{"raw":"# version\nbatch start\nfeature GPS\nset gyro_lpf1_static_hz = 0\nset p_roll = 46\nsave\n"}}`
	env, err := runTestCommandWithInput(t, []string{"configuration", "compare"}, reference, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	summary := env.Data.(map[string]any)["configuration_compare"].(map[string]any)["summary"].(map[string]any)
	if summary["changed_setting_count"] != float64(1) {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestConfigurationExportFullIncludesConfiguration(t *testing.T) {
	env, err := runTestCommand(t, []string{"configuration", "export"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["command"] != "dump all" || data["raw_authoritative"] != true {
		t.Fatalf("data = %+v", data)
	}
	configuration := data["configuration"].(map[string]any)
	if len(configuration["settings"].([]any)) < 8 {
		t.Fatalf("configuration = %+v", configuration)
	}
	inventory := data["inventory"].(map[string]any)
	if inventory["settings"].(float64) < 8 || inventory["serial_ports"].(float64) == 0 {
		t.Fatalf("inventory = %+v", inventory)
	}
}

func TestConfigurationExportDiffRawCLI(t *testing.T) {
	env, err := runTestCommand(t, []string{"configuration", "export", "--source", "diff", "--raw-cli"}, nil)
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
	if !strings.Contains(raw, "feature GPS") || !strings.Contains(raw, "save") {
		t.Fatalf("raw = %q", raw)
	}
}

func TestConfigurationExportRejectsUnknownSource(t *testing.T) {
	env, err := runTestCommand(t, []string{"configuration", "export", "--source", "nope"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want error")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestConfigurationDiffCommandIncludesParsedSettings(t *testing.T) {
	env, err := runTestCommand(t, []string{"configuration", "diff"}, nil)
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
	if len(configuration["settings"].([]any)) < 1 {
		t.Fatalf("configuration = %+v", configuration)
	}
	inventory := data["inventory"].(map[string]any)
	if inventory["settings"].(float64) < 1 {
		t.Fatalf("inventory = %+v", inventory)
	}
}

func fakefcTestConnector(t *testing.T, onConnect func(op connection.OperationClass)) connectFunc {
	t.Helper()
	return func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		if onConnect != nil {
			onConnect(op)
		}
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

func TestConfigurationResetRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"configuration", "reset"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after configuration reset confirmation failure")
	}
}

func TestConfigurationResetWithYesIssuesDefaultsNoSave(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommand(t, []string{"configuration", "reset", "--yes"}, fakefcTestConnector(t, func(op connection.OperationClass) {
		gotOp = op
	}))
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if gotOp != connection.Dangerous {
		t.Fatalf("operation = %v, want Dangerous", gotOp)
	}
	data := env.Data.(map[string]any)
	if data["applied"] != true || data["saved"] != false {
		t.Fatalf("data = %+v", data)
	}
	appliedLines := data["applied_cli_lines"].([]any)
	if len(appliedLines) != 1 || appliedLines[0] != "defaults nosave" {
		t.Fatalf("applied lines = %+v", appliedLines)
	}
	changePlan := data["change_plan"].(map[string]any)
	if changePlan["cli_lines"].([]any)[0] != "defaults nosave" {
		t.Fatalf("change plan = %+v", changePlan)
	}
	foundBackupWarning := false
	for _, warning := range env.Warnings {
		if strings.Contains(warning.Message, "backup create --raw-cli") {
			foundBackupWarning = true
		}
	}
	if !foundBackupWarning {
		t.Fatalf("missing backup warning: %+v", env.Warnings)
	}
	sideEffects := env.SideEffects
	if len(sideEffects) != 1 || sideEffects[0].Type != "cli_command" || sideEffects[0].Command != "defaults nosave" {
		t.Fatalf("side effects = %+v", sideEffects)
	}
}

func TestConfigurationResetWithSaveChainsSave(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommand(t, []string{"configuration", "reset", "--save", "--yes"}, fakefcTestConnector(t, func(op connection.OperationClass) {
		gotOp = op
	}))
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if gotOp != connection.Dangerous {
		t.Fatalf("operation = %v, want Dangerous", gotOp)
	}
	data := env.Data.(map[string]any)
	if data["applied"] != true || data["saved"] != true || data["save_requested"] != true {
		t.Fatalf("data = %+v", data)
	}
	if len(data["save_response_lines"].([]any)) == 0 {
		t.Fatalf("save response lines = %+v", data["save_response_lines"])
	}
	if len(env.SideEffects) != 2 {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
	if env.SideEffects[0].Type != "cli_command" || env.SideEffects[0].Command != "defaults nosave" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
	if env.SideEffects[1].Type != "save" || env.SideEffects[1].Command != "save" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}
