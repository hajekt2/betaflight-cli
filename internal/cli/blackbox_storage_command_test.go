package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
)

func TestBlackboxInspectDoesNotConnect(t *testing.T) {
	path := writeTempBlackboxLog(t)
	called := false
	env, err := runTestCommand(t, []string{"blackbox", "inspect", path}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	inspection := data["inspection"].(map[string]any)
	if inspection["product"] == "" || inspection["firmware_revision"] == "" {
		t.Fatalf("inspection = %+v", inspection)
	}
	decoded := inspection["decoded_frames"].(map[string]any)
	if decoded["decoded_count"] != float64(1) || len(decoded["samples"].([]any)) != 1 {
		t.Fatalf("decoded frames = %+v", decoded)
	}
	streams := decoded["streams"].(map[string]any)
	if streams["I.time"] == nil {
		t.Fatalf("streams = %+v", streams)
	}
	groups := decoded["groups"].(map[string]any)
	if groups["timing"] == nil {
		t.Fatalf("groups = %+v", groups)
	}
	if called {
		t.Fatal("connector was called for offline blackbox inspect")
	}
}

func TestBlackboxInspectOnboardLogIndexWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"blackbox", "inspect", "--log-index", "0"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["log_index"] != float64(0) {
		t.Fatalf("data = %+v", data)
	}
	inspection := data["inspection"].(map[string]any)
	if inspection["product"] == "" || inspection["firmware_revision"] == "" {
		t.Fatalf("inspection = %+v", inspection)
	}
	log := data["log"].(map[string]any)
	if log["index"].(float64) != 0 || log["offset_bytes"].(float64) < 0 || log["size_bytes"].(float64) == 0 {
		t.Fatalf("log = %+v", log)
	}
	if log["product"] == "" || log["firmware_revision"] == "" {
		t.Fatalf("log = %+v", log)
	}
	storage := data["storage"].(map[string]any)
	if storage["dataflash"] == nil {
		t.Fatalf("storage = %+v", storage)
	}
}

func TestBlackboxInspectRejectsOutOfRangeOnboardLogIndex(t *testing.T) {
	env, err := runTestCommand(t, []string{"blackbox", "inspect", "--log-index", "99999"}, nil)
	_ = err
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBlackboxInspectReportsParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.bbl")
	if err := os.WriteFile(path, []byte("not a blackbox log"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	env, err := runTestCommand(t, []string{"blackbox", "inspect", path}, nil)
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "blackbox_parse_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestBlackboxConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"blackbox", "config"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	config := data["blackbox"].(map[string]any)
	if config["supported"] != true || config["device_name"] != "SDCARD" || config["sample_rate_name"] != "1/4" {
		t.Fatalf("config = %+v", config)
	}
	disabled := config["disabled_fields"].([]any)
	if len(disabled) != 2 || disabled[0] != "PID" || disabled[1] != "GPS" {
		t.Fatalf("disabled = %+v", disabled)
	}
}

func TestBlackboxSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"blackbox":{"device":2,"rate_numerator":1,"rate_denominator":4,"p_ratio":16,"sample_rate":2,"fields_disabled_mask":4097}}`
	env, err := runTestCommandWithInput(t, []string{"blackbox", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["blackbox_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["device"] != float64(2) || config["p_ratio"] != float64(16) || result["save_required"] != true {
		t.Fatalf("blackbox_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_BLACKBOX_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBlackboxSetConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"blackbox_config":{"device":2,"rate_numerator":1,"rate_denominator":4,"p_ratio":16,"sample_rate":2,"fields_disabled_mask":4097}}`
	env, err := runTestCommandWithInput(t, []string{"blackbox", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBlackboxListWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"blackbox", "list", "--size", "512"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	list := data["blackbox_logs"].(map[string]any)
	if list["log_count"].(float64) < 1 || list["bytes_read"].(float64) == 0 {
		t.Fatalf("list = %+v", list)
	}
	logs := list["logs"].([]any)
	first := logs[0].(map[string]any)
	if first["product"] == "" || first["firmware_revision"] == "" {
		t.Fatalf("first log = %+v", first)
	}
}

func TestBlackboxExportWithFakeFC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exported.bbl")
	env, err := runTestCommand(t, []string{"blackbox", "export", path, "--size", "64"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	export := data["blackbox_export"].(map[string]any)
	if export["completed"] != true || export["exported_bytes"] != float64(64) || export["path"] != path {
		t.Fatalf("export = %+v", export)
	}
	inspection := data["inspection"].(map[string]any)
	if inspection["product"] == "" || inspection["firmware_revision"] == "" {
		t.Fatalf("inspection = %+v", inspection)
	}
	config := data["blackbox"].(map[string]any)
	if config["supported"] != true {
		t.Fatalf("config = %+v", config)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(content) != 64 || !strings.HasPrefix(string(content), "H Product:Blackbox") {
		t.Fatalf("content = %q", string(content))
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "file_write" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBlackboxExportByLogIndexWithFakeFC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selected.bbl")
	env, err := runTestCommand(t, []string{"blackbox", "export", path, "--size", "512", "--log-index", "0"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	export := data["blackbox_export"].(map[string]any)
	if export["log_index"] != float64(0) || export["exported_bytes"].(float64) == 0 {
		t.Fatalf("export = %+v", export)
	}
	inspection := data["inspection"].(map[string]any)
	if inspection["product"] == "" {
		t.Fatalf("inspection = %+v", inspection)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.HasPrefix(string(content), "H Product:Blackbox") {
		t.Fatalf("content = %q", string(content))
	}
}

func TestBlackboxExportRejectsOutOfRangeLogIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.bbl")
	env, err := runTestCommand(t, []string{"blackbox", "export", path, "--size", "512", "--log-index", "99"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBlackboxExportRequiresForceToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exported.bbl")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	env, err := runTestCommand(t, []string{"blackbox", "export", path, "--size", "8"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want existing-file failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "file_exists" {
		t.Fatalf("env = %+v", env)
	}
}

func TestStorageStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"storage", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	storage := data["storage"].(map[string]any)
	dataflash := storage["dataflash"].(map[string]any)
	if dataflash["ready"] != true || dataflash["free_bytes"] != float64(786432) {
		t.Fatalf("dataflash = %+v", dataflash)
	}
	sdcard := storage["sdcard"].(map[string]any)
	if sdcard["state_name"] != "READY" || sdcard["total_kilobytes"] != float64(32768) {
		t.Fatalf("sdcard = %+v", sdcard)
	}
}

func TestStorageExportWithFakeFC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flash.bbl")
	env, err := runTestCommand(t, []string{"storage", "export", path, "--size", "64"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	export := data["dataflash_export"].(map[string]any)
	if export["completed"] != true || export["exported_bytes"] != float64(64) || export["path"] != path {
		t.Fatalf("export = %+v", export)
	}
	if len(export["chunks"].([]any)) == 0 {
		t.Fatalf("chunks = %+v", export["chunks"])
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(content) != 64 || !strings.HasPrefix(string(content), "H Product:Blackbox") {
		t.Fatalf("content = %q", string(content))
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "file_write" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestStorageExportRequiresForceToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flash.bbl")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	env, err := runTestCommand(t, []string{"storage", "export", path, "--size", "8"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want existing-file failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "file_exists" {
		t.Fatalf("env = %+v", env)
	}
}

func TestStorageEraseRequiresConfirmationBeforeConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"storage", "erase"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if called {
		t.Fatal("connector was called before --yes confirmation")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestStorageEraseUsesDangerousOperation(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommand(t, []string{"storage", "erase", "--yes"}, func(ctx context.Context, cfg connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		gotOp = op
		client, err := connection.NewClient(fakefc.New(), time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(ctx)
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
	plan := data["storage_erase"].(map[string]any)
	if plan["applied"] != true || plan["dangerous"] != true || plan["command"] != "MSP_DATAFLASH_ERASE" {
		t.Fatalf("plan = %+v", plan)
	}
	before := plan["before"].(map[string]any)["dataflash"].(map[string]any)
	after := plan["after"].(map[string]any)["dataflash"].(map[string]any)
	if before["used_bytes"] != float64(262144) || after["used_bytes"] != float64(0) {
		t.Fatalf("before=%+v after=%+v", before, after)
	}
	audit := plan["audit"].(map[string]any)
	if audit["preflight_captured"] != true || audit["post_stop_captured"] != true || audit["freed_bytes"] != float64(262144) || audit["stop_attempted"] != false || audit["stop_succeeded"] != false {
		t.Fatalf("audit = %+v", audit)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "dataflash_erase" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func writeTempBlackboxLog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "log.bbl")
	log := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Firmware revision:Betaflight 2025.12.1 (abc123) STM32F405",
		"H Field I name:loopIteration,time",
		"H Field I signed:0,0",
		"H Field I predictor:6,0",
		"H Field I encoding:1,1",
		"H Field P predictor:0,10",
		"H Field P encoding:0,0",
		"I\x02\x04E",
	}, "\n")
	if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
