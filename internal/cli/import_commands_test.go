package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
)

func TestPlanApplyRequiresYes(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "enable", "gps", "--apply"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
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
	changePlan := data["change_plan"].(map[string]any)
	if changePlan["applied"] != false || len(changePlan["cli_lines"].([]any)) != 2 {
		t.Fatalf("change_plan = %+v", changePlan)
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

func TestBatchPlanRejectsUnknownWriteLine(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"batch", "plan"}, "impossible 1 2 3\n", nil)
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
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
	env, err := runTestCommandWithInput(t, []string{"batch", "apply", "--yes"}, "feature GPS\nset gyro_lpf1_static_hz = 0\n", nil)
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

func TestBatchApplyRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"batch", "apply"}, "feature GPS\n", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after batch apply confirmation failure")
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
	changePlan := data["change_plan"].(map[string]any)
	if changePlan["applied"] != false || len(changePlan["cli_lines"].([]any)) != 2 {
		t.Fatalf("change_plan = %+v", changePlan)
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

func TestRestorePlanAcceptsJSONNoConnect(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"restore", "plan"}, `{"data":{"raw":"# version\nbatch start\ndefaults nosave\nfeature GPS\nsave\nbatch end\n"}}`, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called for restore plan")
	}
	data := env.Data.(map[string]any)
	lines := data["cli_lines"].([]any)
	if len(lines) != 1 || lines[0] != "feature GPS" {
		t.Fatalf("plan = %+v", data)
	}
}

func TestRestoreApplyWithJSONFileAndYes(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommandWithInput(t, []string{"restore", "apply", "--file", "-", "--yes"}, `{"data":{"lines":["feature GPS","set gyro_lpf1_static_hz = 0"]}}`, func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if gotOp != connection.Write {
		t.Fatalf("operation = %v, want Write", gotOp)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["source_format"] != "json" || data["kind"] != "restore" || data["applied"] != true {
		t.Fatalf("plan = %+v", data)
	}
}

func TestRestoreApplyRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"restore", "apply"}, "feature GPS\n", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	env, err := runTestCommandWithInput(t, []string{"restore", "apply", "--yes"}, "feature GPS\nset gyro_lpf1_static_hz = 0\n", nil)
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
	changePlan := data["change_plan"].(map[string]any)
	if changePlan["applied"] != true || len(changePlan["cli_lines"].([]any)) != 2 {
		t.Fatalf("change_plan = %+v", changePlan)
	}
	if len(env.SideEffects) != 2 {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRestoreApplyReportsPartialSideEffects(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"restore", "apply", "--yes"}, "feature GPS\nset gyro_lpf1_static_hz = 0\n", failingCLIConnector(2))
	if err == nil || env.OK {
		t.Fatalf("expected failed partial apply: env=%+v err=%v", env, err)
	}
	data := env.Data.(map[string]any)
	if data["partially_applied"] != true || data["failed_cli_line"] != "set gyro_lpf1_static_hz = 0" {
		t.Fatalf("partial apply data = %+v", data)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "feature GPS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRestoreApplyAllowUnsupportedAddsWarning(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"--allow-unsupported", "restore", "apply", "--yes"}, "feature GPS\n", func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		client, clientErr := connection.NewClient(fakefc.New(), time.Second)
		if clientErr != nil {
			return nil, connection.TargetInfo{}, clientErr
		}
		return client, connection.TargetInfo{Port: "fake", Variant: "INAV", FirmwareVersion: "2025.12.1", MSPAPIVersion: "1.48"}, nil
	})
	if err != nil || !env.OK {
		t.Fatalf("unexpected result: env=%+v err=%v", env, err)
	}
	if len(env.Warnings) == 0 || env.Warnings[len(env.Warnings)-1].Code != "unsupported_firmware" {
		t.Fatalf("warnings = %+v, want unsupported_firmware", env.Warnings)
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
	changePlan := data["change_plan"].(map[string]any)
	if changePlan["kind"] != "preset" || len(changePlan["cli_lines"].([]any)) != 2 {
		t.Fatalf("change_plan = %+v", changePlan)
	}
}

func TestPresetsFetchPlanWithoutApply(t *testing.T) {
	presetBody := "feature GPS\nset small_angle = 25\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("ETag", "\"abc-123\"")
		w.Header().Set("Last-Modified", "Mon, 01 Jan 2024 12:00:00 GMT")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(presetBody)); err != nil {
			t.Fatalf("write response = %v", err)
		}
	}))
	defer server.Close()

	called := false
	env, err := runTestCommandWithInput(t, []string{"presets", "fetch", server.URL}, "", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called for presets fetch plan")
	}
	data := env.Data.(map[string]any)
	if data["applied"] != false || data["kind"] != "preset" {
		t.Fatalf("data = %+v", data)
	}
	plan := data["plan"].(map[string]any)
	lines := plan["cli_lines"].([]any)
	if len(lines) != 2 || lines[0] != "feature GPS" || lines[1] != "set small_angle = 25" {
		t.Fatalf("plan = %+v", plan)
	}
	changePlan := data["change_plan"].(map[string]any)
	changePlanLines := changePlan["cli_lines"].([]any)
	if len(changePlanLines) != 2 || changePlanLines[0] != "feature GPS" || changePlanLines[1] != "set small_angle = 25" {
		t.Fatalf("change_plan = %+v", changePlan)
	}
	source := data["source"].(map[string]any)
	if source["url"] != server.URL || source["http_status"].(float64) != float64(http.StatusOK) || source["content_type"] != "text/plain" {
		t.Fatalf("source = %+v", source)
	}
	if source["checksum_sha256"] == "" || len(source["checksum_sha256"].(string)) != 64 {
		t.Fatalf("source = %+v", source)
	}
	if source["content_length"].(float64) != float64(len(presetBody)) {
		t.Fatalf("source = %+v", source)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "network_fetch" || env.SideEffects[0].Command != server.URL {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestPresetsFetchRejectsBadURLScheme(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"presets", "fetch", "file:///tmp/preset.cli"}, "", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called for bad URL preset fetch")
	}
}

func TestPresetsFetchHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		if _, err := w.Write([]byte("error")); err != nil {
			t.Fatalf("write response = %v", err)
		}
	}))
	defer server.Close()

	called := false
	env, err := runTestCommandWithInput(t, []string{"presets", "fetch", server.URL}, "", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	source := env.Data.(map[string]any)["source"].(map[string]any)
	if source["url"] != server.URL || source["http_status"].(float64) != float64(http.StatusBadGateway) {
		t.Fatalf("source = %+v", source)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "network_fetch" || env.SideEffects[0].Command != server.URL {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
	if called {
		t.Fatal("connector was called for failed preset fetch")
	}
}

func TestPresetsFetchValidationErrorIncludesSource(t *testing.T) {
	presetBody := "save\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(presetBody)); err != nil {
			t.Fatalf("write response = %v", err)
		}
	}))
	defer server.Close()

	called := false
	env, err := runTestCommandWithInput(t, []string{"presets", "fetch", server.URL}, "", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	source := env.Data.(map[string]any)["source"].(map[string]any)
	if source["url"] != server.URL || source["checksum_sha256"] == "" {
		t.Fatalf("source = %+v", source)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "network_fetch" || env.SideEffects[0].Command != server.URL {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
	if called {
		t.Fatal("connector was called after fetched preset validation error")
	}
}

func TestPresetsFetchApplyUsesConnection(t *testing.T) {
	presetBody := "feature GPS\nset small_angle = 25\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(presetBody)); err != nil {
			t.Fatalf("write response = %v", err)
		}
	}))
	defer server.Close()

	var op connection.OperationClass
	env, err := runTestCommandWithInput(t, []string{"presets", "fetch", server.URL, "--apply", "--yes"}, "", func(_ context.Context, _ connection.Config, got connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		op = got
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
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if op != connection.Write {
		t.Fatalf("operation = %v, want Write", op)
	}
	data := env.Data.(map[string]any)
	if data["applied"] != true {
		t.Fatalf("data = %+v", env.Data)
	}
	source := data["source"].(map[string]any)
	if source["url"] != server.URL || source["checksum_sha256"] == "" {
		t.Fatalf("source = %+v", source)
	}
	if len(env.SideEffects) != 3 || env.SideEffects[0].Type != "network_fetch" || env.SideEffects[1].Type != "cli_command" || env.SideEffects[2].Type != "cli_command" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestPresetsFetchApplyRequiresYesDoesNotConnect(t *testing.T) {
	presetBody := "feature GPS\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(presetBody)); err != nil {
			t.Fatalf("write response = %v", err)
		}
	}))
	defer server.Close()

	called := false
	env, err := runTestCommandWithInput(t, []string{"presets", "fetch", server.URL, "--apply"}, "", func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	source := env.Data.(map[string]any)["source"].(map[string]any)
	if source["url"] != server.URL || source["checksum_sha256"] == "" {
		t.Fatalf("source = %+v", source)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "network_fetch" || env.SideEffects[0].Command != server.URL {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
	if called {
		t.Fatal("connector was called after preset fetch confirmation failure")
	}
}
