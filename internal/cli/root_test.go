package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
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

func TestCapabilitiesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"capabilities"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called for capabilities")
	}
	data := env.Data.(map[string]any)
	capabilities := data["capabilities"].(map[string]any)
	commands := capabilities["commands"].([]any)
	if len(commands) < 100 {
		t.Fatalf("commands = %d", len(commands))
	}
	byCommand := map[string]map[string]any{}
	for _, item := range commands {
		command := item.(map[string]any)
		byCommand[command["command"].(string)] = command
	}
	save := byCommand["betaflight-cli save"]
	if save["operation"] != "dangerous" || save["confirmation"] != "--yes" || save["requires_connection"] != true || save["runnable"] != true {
		t.Fatalf("save capability = %+v", save)
	}
	configurationDiff := byCommand["betaflight-cli configuration diff"]
	if configurationDiff["operation"] != "read_only" || configurationDiff["requires_connection"] != true || configurationDiff["confirmation"] != "none" || configurationDiff["runnable"] != true {
		t.Fatalf("configuration diff capability = %+v", configurationDiff)
	}
	settingsDiff := byCommand["betaflight-cli settings diff"]
	if settingsDiff["operation"] != "read_only" || settingsDiff["requires_connection"] != true || settingsDiff["confirmation"] != "none" || settingsDiff["runnable"] != true {
		t.Fatalf("settings diff capability = %+v", settingsDiff)
	}
	beeperEnable := byCommand["betaflight-cli beeper enable"]
	if beeperEnable["operation"] != "plan_or_write" || beeperEnable["requires_connection"] != true || beeperEnable["confirmation"] != "--yes with --apply or --save" || beeperEnable["runnable"] != true {
		t.Fatalf("beeper enable capability = %+v", beeperEnable)
	}
	beeperDisable := byCommand["betaflight-cli beeper disable"]
	if beeperDisable["operation"] != "plan_or_write" || beeperDisable["requires_connection"] != true || beeperDisable["confirmation"] != "--yes with --apply or --save" || beeperDisable["runnable"] != true {
		t.Fatalf("beeper disable capability = %+v", beeperDisable)
	}
	transponderSetProvider := byCommand["betaflight-cli transponder set-provider"]
	if transponderSetProvider["operation"] != "plan_or_write" || transponderSetProvider["requires_connection"] != true || transponderSetProvider["confirmation"] != "--yes with --apply or --save" || transponderSetProvider["runnable"] != true {
		t.Fatalf("transponder set-provider capability = %+v", transponderSetProvider)
	}
	transponderSetData := byCommand["betaflight-cli transponder set-data"]
	if transponderSetData["operation"] != "plan_or_write" || transponderSetData["requires_connection"] != true || transponderSetData["confirmation"] != "--yes with --apply or --save" || transponderSetData["runnable"] != true {
		t.Fatalf("transponder set-data capability = %+v", transponderSetData)
	}
	validate := byCommand["betaflight-cli configuration validate"]
	if validate["operation"] != "offline" || validate["requires_connection"] != false || validate["output_root"] != "configuration_validation" {
		t.Fatalf("validate capability = %+v", validate)
	}
	configuration := byCommand["betaflight-cli configuration"]
	if configuration["operation"] != "group" || configuration["runnable"] != false {
		t.Fatalf("configuration capability = %+v", configuration)
	}
	presetsFetch := byCommand["betaflight-cli presets fetch"]
	if presetsFetch["operation"] != "offline" || presetsFetch["requires_connection"] != false || presetsFetch["confirmation"] != "none" || presetsFetch["output_root"] != "change_plan" {
		t.Fatalf("presets fetch capability = %+v", presetsFetch)
	}
	workflows := capabilities["workflows"].([]any)
	foundBackup := false
	for _, item := range workflows {
		workflow := item.(map[string]any)
		if workflow["name"] == "backup-before-change" && workflow["safety_class"] == "read_only" {
			foundBackup = true
		}
	}
	if !foundBackup {
		t.Fatalf("workflows = %+v", workflows)
	}
}

func TestCapabilitiesCoverageReportsParityDomains(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"capabilities", "coverage"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called for capabilities coverage")
	}
	coverage := env.Data.(map[string]any)["coverage"].(map[string]any)
	summary := coverage["summary"].(map[string]any)
	if summary["domain_count"].(float64) < 15 || summary["implemented_count"].(float64) < 10 {
		t.Fatalf("summary = %+v", summary)
	}
	domains := coverage["domains"].([]any)
	byDomain := map[string]map[string]any{}
	for _, item := range domains {
		domain := item.(map[string]any)
		byDomain[domain["domain"].(string)] = domain
	}
	backup := byDomain["configuration-backup"]
	if backup["status"] != "implemented" || len(backup["read_commands"].([]any)) == 0 {
		t.Fatalf("backup domain = %+v", backup)
	}
	maintenance := byDomain["firmware-maintenance"]
	if maintenance["status"] != "implemented" || len(maintenance["dangerous_commands"].([]any)) == 0 {
		t.Fatalf("maintenance domain = %+v", maintenance)
	}
	motors := byDomain["motors-servos-mixer"]
	if motors["status"] != "implemented" {
		t.Fatalf("motors domain = %+v", motors)
	}
	gaps := coverage["next_gaps"].([]any)
	foundMotorTesting := false
	for _, item := range gaps {
		gap := item.(map[string]any)
		if gap["domain"] == "motor-testing" {
			foundMotorTesting = true
		}
	}
	if !foundMotorTesting {
		t.Fatalf("gaps = %+v", gaps)
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

func TestTelemetryDefaultCommandMapsToSnapshot(t *testing.T) {
	env, err := runTestCommand(t, []string{"telemetry"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("env.Data type = %T", env.Data)
	}
	sources, ok := data["sources"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected telemetry sources: %+v", data["sources"])
	}
	if sources["status"] == nil || sources["attitude"] == nil || sources["battery"] == nil || sources["rc"] == nil {
		t.Fatalf("missing telemetry source keys: %+v", sources)
	}
}

func TestDebugStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"debug", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	debug := data["debug"].(map[string]any)
	values := debug["debug_values"].([]any)
	if len(values) != 8 || values[0] != float64(-1) || values[7] != float64(8) {
		t.Fatalf("debug values = %+v", values)
	}
	trim := debug["accelerometer_trim"].(map[string]any)
	if trim["pitch"] != float64(-12) || trim["roll"] != float64(34) {
		t.Fatalf("trim = %+v", trim)
	}
}

func TestEnvironmentStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"environment", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	environment := data["environment"].(map[string]any)
	altitude := environment["altitude"].(map[string]any)
	if altitude["altitude_m"] != 123.45 || altitude["vario_cm_s"] != float64(-67) {
		t.Fatalf("altitude = %+v", altitude)
	}
	rangefinder := environment["rangefinder"].(map[string]any)
	if rangefinder["altitude_m"] != 43.21 {
		t.Fatalf("rangefinder = %+v", rangefinder)
	}
	analog := environment["analog"].(map[string]any)
	if analog["voltage_v"] != 15.99 || analog["rssi"] != float64(900) || analog["amperage_a"] != -1.23 {
		t.Fatalf("analog = %+v", analog)
	}
}

func TestRTCStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"rtc", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	rtc := data["rtc"].(map[string]any)
	if rtc["available"] != true || rtc["iso_utc"] != "2026-06-23T12:34:56.789Z" || rtc["millis"] != float64(789) {
		t.Fatalf("rtc = %+v", rtc)
	}
}

func TestStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	status := data["status"].(map[string]any)
	runtime := status["runtime"].(map[string]any)
	if runtime["source"] != "MSP_STATUS_EX" || runtime["cycle_time_us"] != float64(250) || runtime["active_sensors"] != float64(33) {
		t.Fatalf("runtime = %+v", runtime)
	}
	health := status["health"].(map[string]any)
	if health["cpu_load_percent"] != float64(42) || health["i2c_errors_present"] != false {
		t.Fatalf("health = %+v", health)
	}
	activeNames := status["active_sensor_names"].([]any)
	if len(activeNames) != 2 || activeNames[0] != "accelerometer" || activeNames[1] != "gyro" {
		t.Fatalf("active_sensor_names = %+v", activeNames)
	}
	flightModes := status["flight_modes"].(map[string]any)
	modeNames := flightModes["active_names"].([]any)
	if len(modeNames) != 1 || modeNames[0] != "ANGLE" {
		t.Fatalf("flight mode names = %+v", modeNames)
	}
	arming := status["arming"].(map[string]any)
	if arming["source"] != "MSP_STATUS_EX" {
		t.Fatalf("arming = %+v", arming)
	}
}

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

func TestTasksStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"tasks", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	tasks := data["tasks"].(map[string]any)
	rows := tasks["tasks"].([]any)
	if len(rows) != 2 {
		t.Fatalf("tasks = %+v", rows)
	}
	pid := rows[1].(map[string]any)
	if pid["name"] != "PID" || pid["rate_hz"] != float64(8000) || pid["late_count"] != float64(1) {
		t.Fatalf("pid row = %+v", pid)
	}
	total := tasks["total"].(map[string]any)
	if total["average_load_percent"] != 1.6 {
		t.Fatalf("total = %+v", total)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "task_stats_reset" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSystemStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"system", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	system := data["system"].(map[string]any)
	config := system["config"].(map[string]any)
	if config["state"] != "CONFIGURED" || config["used_bytes"] != float64(3820) {
		t.Fatalf("config = %+v", config)
	}
	runtime := system["runtime"].(map[string]any)
	if runtime["cpu_load_percent"] != float64(42) || runtime["gyro_rate_hz"] != float64(4000) {
		t.Fatalf("runtime = %+v", runtime)
	}
	voltage := system["voltage"].(map[string]any)
	if voltage["voltage_v"] != 15.99 || voltage["cell_count"] != float64(4) {
		t.Fatalf("voltage = %+v", voltage)
	}
	arming := system["arming"].(map[string]any)
	flags := arming["flags"].([]any)
	if len(flags) != 2 || flags[0] != "RXLOSS" {
		t.Fatalf("arming = %+v", arming)
	}
}

func TestResourcesStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"resources", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	resources := data["resources"].(map[string]any)
	assignments := resources["resources"].([]any)
	if len(assignments) != 2 {
		t.Fatalf("resources = %+v", assignments)
	}
	first := assignments[0].(map[string]any)
	if first["kind"] != "MOTOR" || first["index"] != "1" || first["target"] != "A00" {
		t.Fatalf("first resource = %+v", first)
	}
	timers := resources["timers"].([]any)
	if len(timers) != 2 {
		t.Fatalf("timers = %+v", timers)
	}
	timer := timers[1].(map[string]any)
	if timer["pin"] != "A09" || timer["none"] != true {
		t.Fatalf("none timer = %+v", timer)
	}
	dma := resources["dma"].([]any)
	if len(dma) != 2 {
		t.Fatalf("dma = %+v", dma)
	}
	deviceDMA := dma[1].(map[string]any)
	if deviceDMA["scope"] != "SPI_TX" || deviceDMA["device"] != "1" || deviceDMA["option"] != "0" {
		t.Fatalf("device dma = %+v", deviceDMA)
	}
}

func TestInfoWithFakeFCIncludesBuildMetadata(t *testing.T) {
	env, err := runTestCommand(t, []string{"info"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["legacy_name"] != "BetaFlight" {
		t.Fatalf("legacy name = %+v", data["legacy_name"])
	}
	support := data["support"].(map[string]any)
	if support["supported"] != true {
		t.Fatalf("support = %+v", support)
	}
	if support["policy"] != "official Betaflight 2025.12.x and newer" {
		t.Fatalf("support policy = %+v", support)
	}
	board := data["board"].(map[string]any)
	if board["configuration_state_name"] != "CONFIGURED" || board["sample_rate_hz"] != float64(8000) {
		t.Fatalf("board = %+v", board)
	}
	problems := board["configuration_problems"].(map[string]any)
	if problems["mask"] != float64(3) {
		t.Fatalf("configuration problems = %+v", problems)
	}
	mcu := data["mcu"].(map[string]any)
	if mcu["source"] != "MSP2_MCU_INFO" || mcu["id"] != float64(254) || mcu["name"] != "STM32F405" {
		t.Fatalf("mcu = %+v", mcu)
	}
	uid := data["uid"].(map[string]any)
	if uid["source"] != "MSP_UID" || uid["hex"] != "0123456789abcdef00000042" {
		t.Fatalf("uid = %+v", uid)
	}
	build := data["build"].(map[string]any)
	if build["date_time"] != "Jan 01 2026 00:00:00" || build["git_revision"] != "abc1234" {
		t.Fatalf("build = %+v", build)
	}
	names := build["build_option_names"].([]any)
	if len(names) != 2 || names[0] != "USE_GPS" || names[1] != "USE_DSHOT" {
		t.Fatalf("build option names = %+v", names)
	}
	unknown := build["unknown_options"].([]any)
	if len(unknown) != 1 || unknown[0].(map[string]any)["code"] != float64(65000) {
		t.Fatalf("unknown options = %+v", unknown)
	}
}

func TestFirmwareStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"firmware", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	firmware := data["firmware"].(map[string]any)
	if firmware["variant"] != "BTFL" || firmware["version"] != "2025.12.1" || firmware["msp_api"] != "1.48" {
		t.Fatalf("firmware identity = %+v", firmware)
	}
	support := firmware["support"].(map[string]any)
	if support["supported"] != true || support["policy"] != "official Betaflight 2025.12.x and newer" {
		t.Fatalf("support = %+v", support)
	}
	target := firmware["target"].(map[string]any)
	if target["target_name"] != "STM32F405" || target["board_name"] != "FAKEF405" {
		t.Fatalf("target = %+v", target)
	}
	metadata := firmware["metadata"].(map[string]any)
	if metadata["settings_source_firmware"] != "2025.12.0" || metadata["settings_count"].(float64) < 100 {
		t.Fatalf("metadata = %+v", metadata)
	}
	capabilities := firmware["capabilities"].(map[string]any)
	if capabilities["sample_rate_hz"] != float64(8000) {
		t.Fatalf("capabilities = %+v", capabilities)
	}
	identity := firmware["identity"].(map[string]any)
	if identity["configurator_uid"] != "123456789abcdef42" || identity["configuration_state_name"] != "CONFIGURED" {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestTargetStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"target", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	target := data["target"].(map[string]any)
	summary := target["summary"].(map[string]any)
	if summary["supported"] != true || summary["firmware_version"] != "2025.12.1" || summary["target_name"] != "STM32F405" {
		t.Fatalf("summary = %+v", summary)
	}
	if summary["board_name"] != "FAKEF405" || summary["configuration_state"] != "CONFIGURED" || summary["build_key"] != "fake-build-key" {
		t.Fatalf("summary identity = %+v", summary)
	}
	if summary["resource_count"] != float64(2) || summary["timer_count"] != float64(2) || summary["dma_count"] != float64(2) {
		t.Fatalf("summary counts = %+v", summary)
	}
	if target["firmware"] == nil || target["system"] == nil || target["resources"] == nil {
		t.Fatalf("target sections = %+v", target)
	}
}

func TestProbeSupportAndMetadata(t *testing.T) {
	support := probeSupport(connection.TargetInfo{
		Variant:         "BTFL",
		FirmwareVersion: "2025.12.1",
		MSPAPIVersion:   "1.48",
	})
	if !support.Supported || support.Reason != "firmware is inside the supported metadata range" {
		t.Fatalf("support = %+v", support)
	}
	metadata := probeMetadata()
	if metadata["settings_source_firmware"] != "2025.12.0" || metadata["settings_count"].(int) < 100 {
		t.Fatalf("metadata = %+v", metadata)
	}
	files := metadata["settings_source_files"].([]string)
	if len(files) == 0 || files[0] != "src/main/cli/settings.c" {
		t.Fatalf("settings_source_files = %+v", files)
	}
}

func TestDiagnosePortsRecommendations(t *testing.T) {
	tests := []struct {
		name              string
		ports             []connection.PortInfo
		candidateCount    int
		singleCandidate   bool
		recommendedPort   string
		recommendedAction string
		warningCount      int
	}{
		{
			name: "none",
			ports: []connection.PortInfo{
				{Name: "/dev/cu.Bluetooth-Incoming-Port", Candidate: false, Reason: "ignored non-USB or debug port"},
			},
			recommendedAction: "connect a Betaflight flight controller over USB, then run doctor --probe",
			warningCount:      1,
		},
		{
			name: "single",
			ports: []connection.PortInfo{
				{Name: "/dev/cu.usbmodem01", Candidate: true, Reason: "macOS USB serial candidate"},
			},
			candidateCount:    1,
			singleCandidate:   true,
			recommendedPort:   "/dev/cu.usbmodem01",
			recommendedAction: "run doctor --probe or use this port for read-only commands",
		},
		{
			name: "multiple",
			ports: []connection.PortInfo{
				{Name: "/dev/cu.usbmodem01", Candidate: true, Reason: "macOS USB serial candidate"},
				{Name: "/dev/cu.usbmodem02", Candidate: true, Reason: "macOS USB serial candidate"},
			},
			candidateCount:    2,
			recommendedAction: "run doctor --probe or pass --port explicitly after selecting the intended flight controller",
			warningCount:      1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagnosePorts(tt.ports)
			if got.CandidateCount != tt.candidateCount || got.SingleCandidate != tt.singleCandidate || got.RecommendedPort != tt.recommendedPort {
				t.Fatalf("diagnostics = %+v", got)
			}
			if got.RecommendedAction != tt.recommendedAction {
				t.Fatalf("recommended action = %q", got.RecommendedAction)
			}
			if len(got.Warnings) != tt.warningCount {
				t.Fatalf("warnings = %+v", got.Warnings)
			}
		})
	}
}

func TestTextStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"text", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	text := data["text"].(map[string]any)
	if text["source"] != "MSP2_GET_TEXT" {
		t.Fatalf("text = %+v", text)
	}
	byKey := text["by_key"].(map[string]any)
	if byKey["craft_name"] != "BetaFlight" || byKey["pilot_name"] != "BF pilot" || byKey["release_name"] != "2025.12.1" {
		t.Fatalf("by_key = %+v", byKey)
	}
	fields := text["fields"].([]any)
	if len(fields) != 7 {
		t.Fatalf("fields = %+v", fields)
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
	if audit["preflight_captured"] != true || audit["post_stop_captured"] != true || audit["freed_bytes"] != float64(262144) {
		t.Fatalf("audit = %+v", audit)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "dataflash_erase" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMSPRequestByNameReturnsMetadata(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_NAME"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["code"].(float64) != float64(msp.MSPName) {
		t.Fatalf("code = %+v", data["code"])
	}
	if data["code_name"] != "MSP_NAME" {
		t.Fatalf("code_name = %+v", data["code_name"])
	}
	if data["payload_hex"].(string) != "42657461466c69676874" {
		t.Fatalf("payload_hex = %+v", data["payload_hex"])
	}
	if data["command_source"].(string) != "src/main/msp/msp_protocol.h" {
		t.Fatalf("command_source = %+v", data["command_source"])
	}
	if data["direction_hint"] != "read" {
		t.Fatalf("direction_hint = %+v", data["direction_hint"])
	}
}

func TestMSPRequestNumericFallbackUsesCodeName(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "0xFFFF"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["code"].(float64) != 65535 {
		t.Fatalf("code = %+v", data["code"])
	}
	if data["code_name"] != "MSP_65535" {
		t.Fatalf("code_name = %+v", data["code_name"])
	}
	if _, ok := data["command_source"].(string); !ok || data["command_source"] != "" {
		t.Fatalf("command_source = %+v", data["command_source"])
	}
}

func TestMSPRequestWithDecodeReturnsStructuredPayload(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_NAME", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	if data["decode_requested"] != true {
		t.Fatalf("decode_requested = %v", data["decode_requested"])
	}
	if data["decoded"] != "BetaFlight" {
		t.Fatalf("decoded = %v", data["decoded"])
	}
}

func TestMSPRequestWithDecodeUnsupportedCodeFallsBackToRaw(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "0xFFFF", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["decode_supported"] != false {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	if data["decode_requested"] != true {
		t.Fatalf("decode_requested = %v", data["decode_requested"])
	}
	if _, ok := data["decoded"]; ok {
		t.Fatalf("decoded should be absent for unsupported code: %+v", data["decoded"])
	}
}

func TestMSPRequestWithDecodeForBatteryProfile(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_BATTERY_PROFILE", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["index"] != float64(0) {
		t.Fatalf("decoded index = %v", decoded["index"])
	}
	if decoded["capacity_mah"] != float64(420) {
		t.Fatalf("decoded capacity_mah = %v", decoded["capacity_mah"])
	}
	if decoded["force_cell_count"] != float64(4) {
		t.Fatalf("decoded force_cell_count = %v", decoded["force_cell_count"])
	}
}

func TestMSPRequestWithDecodeForLEDStripConfig(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_LED_STRIP_CONFIG", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["profile"] != float64(3) {
		t.Fatalf("decoded profile = %v", decoded["profile"])
	}
	if decoded["advanced_supported"] != true {
		t.Fatalf("decoded advanced_supported = %v", decoded["advanced_supported"])
	}
	leds := decoded["leds"].([]any)
	if len(leds) < 2 {
		t.Fatalf("decoded leds = %+v", leds)
	}
}

func TestMSPRequestWithDecodeForGPSRescue(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_GPS_RESCUE", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["max_rescue_angle"] != float64(3200) {
		t.Fatalf("decoded max_rescue_angle = %v", decoded["max_rescue_angle"])
	}
	if decoded["return_altitude_m"] != float64(100) || decoded["descent_distance_m"] != float64(50) {
		t.Fatalf("decoded altitude fields = %+v", decoded)
	}
	if decoded["sanity_checks"] != float64(1) || decoded["min_sats"] != float64(8) {
		t.Fatalf("decoded rescue options = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForGPSHomeFromCompGPS(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_COMP_GPS", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["distance_m"] != float64(342) || decoded["direction_degrees"] != float64(184) || decoded["update"] != true {
		t.Fatalf("decoded home = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForGPSRescuePIDs(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_GPS_RESCUE_PIDS", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["altitude_p"] != float64(80) || decoded["velocity_p"] != float64(120) || decoded["yaw_p"] != float64(45) {
		t.Fatalf("decoded rescue pid = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForDebugValues(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_DEBUG", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].([]any)
	if len(decoded) != 8 || decoded[0] != float64(-1) || decoded[7] != float64(8) {
		t.Fatalf("decoded debug values = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForText(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_GET_TEXT", "--decode", "--payload-hex", "01"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["type"] != float64(1) {
		t.Fatalf("decoded type = %v", decoded["type"])
	}
	if decoded["value"] != "BF pilot" {
		t.Fatalf("decoded value = %v", decoded["value"])
	}
	if decoded["text_length"] != float64(7) {
		t.Fatalf("decoded text_length = %v", decoded["text_length"])
	}
}

func TestMSPMetadataByName(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "metadata", "MSP_NAME"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["code"].(float64) != float64(msp.MSPName) {
		t.Fatalf("code = %+v", data["code"])
	}
	if data["code_name"] != "MSP_NAME" {
		t.Fatalf("code_name = %+v", data["code_name"])
	}
	if data["direction"] != string(msp.DirectionRead) {
		t.Fatalf("direction = %+v", data["direction"])
	}
}

func TestMSPListReturnsCommands(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "list", "--name", "MSP_NAME", "--direction", "read"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["count"].(float64) == 0 {
		t.Fatalf("count = %+v", data["count"])
	}
	commands := data["commands"].([]any)
	if len(commands) == 0 {
		t.Fatalf("commands = %+v", data["commands"])
	}
	first := commands[0].(map[string]any)
	if first["name"].(string) != "MSP_NAME" {
		t.Fatalf("first name = %+v", first["name"])
	}
}

func TestMSPListCanFilterByProtocol(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "list", "--protocol", "1", "--name", "MSP_NAME"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := env.Data.(map[string]any)
	commands := data["commands"].([]any)
	if len(commands) != 1 {
		t.Fatalf("commands = %+v", data["commands"])
	}
	command := commands[0].(map[string]any)
	if command["protocol"].(float64) != 1 {
		t.Fatalf("protocol = %+v", command["protocol"])
	}
}

func TestMSPRequestRequiresYesForWriteLikeCommands(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_SET_NAME", "--payload-hex", "466f6f"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestVTXConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "config"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	config := data["vtx"].(map[string]any)
	if config["type_name"] != "SMARTAUDIO" || config["frequency_mhz"] != float64(5861) || config["pit_mode"] != true {
		t.Fatalf("config = %+v", config)
	}
	table := config["table"].(map[string]any)
	if table["available"] != true || table["bands"] != float64(5) || table["channels"] != float64(8) || table["power_levels"] != float64(3) {
		t.Fatalf("table = %+v", table)
	}
}

func TestModesActiveWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"modes", "active"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	modes := data["modes"].(map[string]any)
	definitions := modes["definitions"].([]any)
	if len(definitions) != 34 {
		t.Fatalf("definitions = %+v", definitions)
	}
	last := definitions[33].(map[string]any)
	if last["id"] != float64(53) || last["name"] != "READY" {
		t.Fatalf("last definition = %+v", last)
	}
	ranges := modes["ranges"].([]any)
	if len(ranges) != 3 {
		t.Fatalf("ranges = %+v", ranges)
	}
	arm := ranges[0].(map[string]any)
	if arm["name"] != "ARM" || arm["aux_channel_name"] != "AUX1" || arm["active"] != true {
		t.Fatalf("arm range = %+v", arm)
	}
	beeperMute := ranges[1].(map[string]any)
	if beeperMute["name"] != "BEEPER MUTE" || beeperMute["mode_logic_name"] != "AND" || beeperMute["linked_to_name"] != "READY" {
		t.Fatalf("beeper mute range = %+v", beeperMute)
	}
}

func TestReceiverStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	receiver := data["receiver"].(map[string]any)
	config := receiver["config"].(map[string]any)
	if config["serial_provider"] != float64(2) || config["stick_center"] != float64(1500) || config["rx_min_usec"] != float64(885) {
		t.Fatalf("config = %+v", config)
	}
	if receiver["rssi_channel"] != float64(8) {
		t.Fatalf("receiver = %+v", receiver)
	}
	channels := receiver["channels"].([]any)
	if len(channels) != 6 || channels[3] != float64(1000) {
		t.Fatalf("channels = %+v", channels)
	}
	rcMapNames := receiver["rc_map_names"].([]any)
	if len(rcMapNames) != 4 || rcMapNames[2] != "THROTTLE" || rcMapNames[3] != "YAW" {
		t.Fatalf("rc map names = %+v", rcMapNames)
	}
	failsafe := receiver["failsafe"].([]any)
	if len(failsafe) != 4 {
		t.Fatalf("failsafe = %+v", failsafe)
	}
	row := failsafe[2].(map[string]any)
	if row["mode_name"] != "SET" || row["value"] != float64(1100) || row["cli_command"] != "rxfail 2 s 1100" {
		t.Fatalf("failsafe row = %+v", row)
	}
}

func TestGPSStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	gps := data["gps"].(map[string]any)
	config := gps["config"].(map[string]any)
	if config["provider"] != float64(1) || config["auto_config"] != true || config["ublox_use_galileo"] != true {
		t.Fatalf("config = %+v", config)
	}
	position := gps["position"].(map[string]any)
	if position["fix"] != true || position["satellites"] != float64(12) || position["latitude_degrees"] != 59.9123456 {
		t.Fatalf("position = %+v", position)
	}
	rescue := gps["rescue"].(map[string]any)
	if rescue["min_sats"] != float64(8) || rescue["initial_climb_m"] != float64(20) {
		t.Fatalf("rescue = %+v", rescue)
	}
	satellites := gps["satellites"].([]any)
	if len(satellites) != 2 {
		t.Fatalf("satellites = %+v", satellites)
	}
}

func TestOSDStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	osd := data["osd"].(map[string]any)
	config := osd["config"].(map[string]any)
	if config["video_system_name"] != "HD" || config["units_name"] != "METRIC" {
		t.Fatalf("config = %+v", config)
	}
	flags := config["flags"].(map[string]any)
	if flags["feature_enabled"] != true || flags["hardware_max7456"] != true || flags["device_detected"] != true {
		t.Fatalf("flags = %+v", flags)
	}
	canvas := osd["canvas"].(map[string]any)
	if canvas["columns"] != float64(53) || canvas["rows"] != float64(20) {
		t.Fatalf("canvas = %+v", canvas)
	}
	warnings := osd["warnings"].(map[string]any)
	if warnings["text"] != "LOW BATTERY" {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestSensorsStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	sensors := data["sensors"].(map[string]any)
	active := sensors["active"].([]any)
	if len(active) != 6 {
		t.Fatalf("active = %+v", active)
	}
	activeGyros := sensors["active_gyros"].(map[string]any)
	if activeGyros["source"] != "MSP2_GYRO_SENSOR_ACTIVE" || activeGyros["count"] != float64(2) {
		t.Fatalf("active gyros = %+v", activeGyros)
	}
	gyroHardware := activeGyros["hardware"].([]any)
	if len(gyroHardware) != 2 || gyroHardware[0].(map[string]any)["name"] != "ICM20689" || gyroHardware[1].(map[string]any)["hardware_id"] != float64(19) {
		t.Fatalf("gyro hardware = %+v", gyroHardware)
	}
	imu := sensors["imu"].(map[string]any)
	accRaw := imu["accelerometer_raw"].([]any)
	if accRaw[0] != float64(2048) || accRaw[1] != float64(-1024) {
		t.Fatalf("imu = %+v", imu)
	}
	alignment := sensors["alignment"].(map[string]any)
	if alignment["gyro_enabled_mask"] != float64(3) {
		t.Fatalf("alignment = %+v", alignment)
	}
	compass := sensors["compass"].(map[string]any)
	if compass["declination_degrees"] != 12.3 {
		t.Fatalf("compass = %+v", compass)
	}
}

func TestBeeperConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "config"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	beeper := data["beeper"].(map[string]any)
	if beeper["dshot_beacon_tone"] != float64(3) || beeper["disabled_mask"] != float64(0x12) {
		t.Fatalf("beeper = %+v", beeper)
	}
	disabled := beeper["disabled"].([]any)
	if len(disabled) != 2 || disabled[0] != "RX_LOST" || disabled[1] != "ARMING" {
		t.Fatalf("disabled = %+v", disabled)
	}
}

func TestBeeperEnablePlanOnly(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "enable", "ARMING"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)
	if plan["kind"] != "beeper" || plan["applied"] != false {
		t.Fatalf("plan = %+v", plan)
	}
	lines := plan["cli_lines"].([]any)
	if len(lines) != 1 || lines[0] != "beeper ARMING" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestBeeperEnableRejectsUnknownMode(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "enable", "NOT_A_MODE"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK {
		t.Fatalf("expected failure: %+v", env)
	}
}

func TestBeeperDisableApply(t *testing.T) {
	var seenOp connection.OperationClass
	connect := func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		seenOp = op
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
	env, err := runTestCommandWithInput(t, []string{"beeper", "disable", "ARMING", "--apply", "--yes"}, "", connect)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if seenOp != connection.Write {
		t.Fatalf("operation = %v, want Write", seenOp)
	}
	plan := env.Data.(map[string]any)
	if plan["applied"] != true || plan["kind"] != "beeper" {
		t.Fatalf("plan = %+v", plan)
	}
	if _, ok := plan["response_lines"]; !ok {
		t.Fatalf("missing response_lines: %+v", plan)
	}
}

func TestTransponderConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "config"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	transponder := data["transponder"].(map[string]any)
	if transponder["source"] != "MSP_TRANSPONDER_CONFIG" || transponder["provider_name"] != "ARCITIMER" || transponder["data_hex"] != "123456789ABCDEF042" {
		t.Fatalf("transponder = %+v", transponder)
	}
	providers := transponder["providers"].([]any)
	if len(providers) != 3 || providers[1].(map[string]any)["data_length"] != float64(9) {
		t.Fatalf("providers = %+v", providers)
	}
	commands := transponder["cli_commands"].([]any)
	if len(commands) != 2 || commands[0] != "set transponder_provider = ARCITIMER" {
		t.Fatalf("cli_commands = %+v", commands)
	}
}

func TestTransponderSetProviderPlan(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-provider", "ARCITIMER"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)
	if plan["kind"] != "transponder" || plan["applied"] != false {
		t.Fatalf("plan = %+v", plan)
	}
	lines := plan["cli_lines"].([]any)
	if len(lines) != 1 || lines[0] != "set transponder_provider = ARCITIMER" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestTransponderSetProviderRejectsUnknown(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-provider", "UNKNOWN"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK {
		t.Fatalf("expected failure: %+v", env)
	}
}

func TestTransponderSetDataApply(t *testing.T) {
	var seenOp connection.OperationClass
	connect := func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		seenOp = op
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
	env, err := runTestCommandWithInput(t, []string{"transponder", "set-data", "1,2,3", "--apply", "--yes"}, "", connect)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if seenOp != connection.Write {
		t.Fatalf("operation = %v, want Write", seenOp)
	}
	plan := env.Data.(map[string]any)
	if plan["applied"] != true || plan["kind"] != "transponder" {
		t.Fatalf("plan = %+v", plan)
	}
	if plan["command_preview"] != "set transponder_data = 1,2,3" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestMixerStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"mixer", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	mixer := data["mixer"].(map[string]any)
	mode := mixer["mixer"].(map[string]any)
	if mode["cli_name"] != "QUADX" || mode["display_name"] != "Quad X" || mode["motor_count"] != float64(4) {
		t.Fatalf("mixer mode = %+v", mode)
	}
	if mixer["yaw_motors_reversed"] != true {
		t.Fatalf("mixer = %+v", mixer)
	}
	commands := mixer["cli_commands"].([]any)
	if len(commands) != 2 || commands[0] != "mixer QUADX" || commands[1] != "set yaw_motors_reversed = ON" {
		t.Fatalf("commands = %+v", commands)
	}
}

func TestMotorsStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	motors := data["motors"].(map[string]any)
	config := motors["config"].(map[string]any)
	if config["max_throttle"] != float64(2000) || config["motor_count"] != float64(4) {
		t.Fatalf("config = %+v", config)
	}
	outputs := motors["outputs"].([]any)
	if len(outputs) != 8 || outputs[3] != float64(1003) {
		t.Fatalf("outputs = %+v", outputs)
	}
	telemetry := motors["telemetry"].([]any)
	first := telemetry[0].(map[string]any)
	if first["rpm"] != float64(12500) || first["voltage_v"] != 16.8 {
		t.Fatalf("telemetry = %+v", telemetry)
	}
}

func TestMotorsTestPlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"motors", "test-plan", "--motor", "2", "--value", "1100", "--duration", "1500ms", "--props-off"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called for motors test-plan")
	}
	plan := env.Data.(map[string]any)["motor_test_plan"].(map[string]any)
	if plan["applied"] != false || plan["dangerous"] != true || plan["command_preview"] != "motor 2 1100" || plan["duration_ms"] != float64(1500) {
		t.Fatalf("plan = %+v", plan)
	}
	checks := plan["safety_checks"].([]any)
	if checks[0].(map[string]any)["passed"] != true || checks[1].(map[string]any)["passed"] != false {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestMotorsTestPlanValidatesBounds(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "test-plan", "--value", "2500"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestMotorsTestApplyRequiresConfirmationBeforeConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"motors", "test-apply", "--motor", "1", "--value", "1050", "--duration", "1ms", "--props-off", "--battery-aware"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestMotorsTestApplyUsesDangerousOperation(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommand(t, []string{"motors", "test-apply", "--motor", "1", "--value", "1050", "--duration", "1ms", "--props-off", "--battery-aware", "--yes"}, func(ctx context.Context, cfg connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	plan := env.Data.(map[string]any)["motor_test_plan"].(map[string]any)
	if plan["applied"] != true || plan["stopped"] != true || plan["command_preview"] != "motor 1 1050" || plan["stop_command_preview"] != "motor 1 1000" {
		t.Fatalf("plan = %+v", plan)
	}
	preflight := plan["preflight"].(map[string]any)
	if preflight["read_only"] != true || preflight["arming_blocked"] != true {
		t.Fatalf("preflight = %+v", preflight)
	}
	postStop := plan["post_stop"].(map[string]any)
	if postStop["read_only"] != true || len(postStop["outputs"].([]any)) == 0 || len(postStop["telemetry_rpm"].([]any)) == 0 {
		t.Fatalf("post stop = %+v", postStop)
	}
	comparison := plan["comparison"].(map[string]any)
	if comparison["read_only"] != true || comparison["preflight_captured"] != true || comparison["post_stop_captured"] != true || comparison["output_count"].(float64) == 0 {
		t.Fatalf("comparison = %+v", comparison)
	}
	audit := plan["audit"].(map[string]any)
	if audit["safety_passed"] != true || audit["preflight_captured"] != true || audit["post_stop_captured"] != true || audit["stop_succeeded"] != true {
		t.Fatalf("audit = %+v", audit)
	}
	if audit["requested_duration_ms"] != float64(1) || audit["elapsed_duration_ms"].(float64) < 0 {
		t.Fatalf("audit timing = %+v", audit)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_output" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestServosStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"servos", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	servos := data["servos"].(map[string]any)
	outputs := servos["outputs"].([]any)
	if len(outputs) != 4 || outputs[1] != float64(1501) {
		t.Fatalf("outputs = %+v", outputs)
	}
	configs := servos["configurations"].([]any)
	first := configs[0].(map[string]any)
	if first["min"] != float64(1000) || first["reversed_sources_mask"] != float64(5) {
		t.Fatalf("configs = %+v", configs)
	}
	rules := servos["mix_rules"].([]any)
	rule := rules[0].(map[string]any)
	if rule["active"] != true || rule["rate"] != float64(100) {
		t.Fatalf("rules = %+v", rules)
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

func TestFeaturesStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	features := data["features"].(map[string]any)
	if features["source"] != "MSP_FEATURE_CONFIG" || features["mask"] != float64(0x00040488) {
		t.Fatalf("features = %+v", features)
	}
	enabled := features["enabled_names"].([]any)
	want := []string{"RX_SERIAL", "GPS", "TELEMETRY", "OSD"}
	if len(enabled) != len(want) {
		t.Fatalf("enabled = %+v", enabled)
	}
	for i, name := range want {
		if enabled[i] != name {
			t.Fatalf("enabled = %+v, want %v", enabled, want)
		}
	}
	catalog := features["catalog"].([]any)
	if len(catalog) != 24 {
		t.Fatalf("catalog length = %d", len(catalog))
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

func TestReceiverRXFailPlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "rxfail", "2", "s", "1100"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if lines[0] != "rxfail 2 s 1100" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only rxfail command")
	}
}

func TestReceiverRXFailValidationFailureDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "rxfail", "4", "a"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after rxfail validation failure")
	}
}

func TestReceiverRXFailApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "rxfail", "2", "s", "1100", "--apply"}, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "rxfail 2 s 1100" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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

func TestProfilesStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"profiles", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	profiles := data["profiles"].(map[string]any)
	if profiles["source"] != "MSP_STATUS_EX" {
		t.Fatalf("profiles = %+v", profiles)
	}
	pid := profiles["pid_profile"].(map[string]any)
	if pid["index"] != float64(0) || pid["count"] != float64(4) || pid["cli_command"] != "profile 0" {
		t.Fatalf("pid profile = %+v", pid)
	}
	rate := profiles["rate_profile"].(map[string]any)
	if rate["index"] != float64(0) || rate["count"] != float64(6) || rate["cli_command"] != "rateprofile 0" {
		t.Fatalf("rate profile = %+v", rate)
	}
	battery := profiles["battery_profile"].(map[string]any)
	if battery["index"] != float64(1) || battery["count"] != float64(3) || battery["cli_command"] != "battery_profile 1" {
		t.Fatalf("battery profile = %+v", battery)
	}
	if profiles["reboot_required"] != false {
		t.Fatalf("profiles = %+v", profiles)
	}
}

func TestProfilesBatterySelectPlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"profiles", "battery-select", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if lines[0] != "battery_profile 1" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only battery profile command")
	}
}

func TestProfilesBatterySelectValidationFailureDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"profiles", "battery-select", "one"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after battery profile validation failure")
	}
}

func TestProfilesBatterySelectApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"profiles", "battery-select", "1", "--apply"}, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "battery_profile 1" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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
	if called {
		t.Fatal("connector was called for failed preset fetch")
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
	if env.Data.(map[string]any)["applied"] != true {
		t.Fatalf("data = %+v", env.Data)
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

func TestSerialListIncludesDecodedFunctions(t *testing.T) {
	env, err := runTestCommand(t, []string{"serial", "list"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	view := data["view"].(map[string]any)
	serial := view["serial"].([]any)
	first := serial[0].(map[string]any)
	functions := first["functions"].([]any)
	if first["function_mask_value"] != float64(64) || functions[0] != "RX_SERIAL" || first["msp_baudrate"] != "115200" {
		t.Fatalf("serial view = %+v", first)
	}
}

func TestAdjustmentsStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"adjustments", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	adjustments := data["adjustments"].(map[string]any)
	if adjustments["source"] != "MSP_ADJUSTMENT_RANGES" {
		t.Fatalf("adjustments = %+v", adjustments)
	}
	ranges := adjustments["ranges"].([]any)
	if len(ranges) != 2 {
		t.Fatalf("ranges = %+v", ranges)
	}
	second := ranges[1].(map[string]any)
	if second["adjustment_function_name"] != "BATTERY_PROFILE" || second["range_start_us"] != float64(1000) || second["cli_command"] != "adjrange 1 0 1 1000 1100 33 2 1600 50" {
		t.Fatalf("second range = %+v", second)
	}
}

func TestSerialStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"serial", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	status := data["serial"].(map[string]any)
	if status["source"] != "MSP2_COMMON_SERIAL_CONFIG" {
		t.Fatalf("serial status = %+v", status)
	}
	ports := status["ports"].([]any)
	if len(ports) != 2 {
		t.Fatalf("ports = %+v", ports)
	}
	first := ports[0].(map[string]any)
	if first["identifier_name"] != "USB_VCP" || first["msp_baudrate"] != "115200" {
		t.Fatalf("first = %+v", first)
	}
	second := ports[1].(map[string]any)
	functions := second["functions"].([]any)
	if second["identifier_name"] != "UART1" || second["function_mask"] != float64(66) || functions[0] != "GPS" || functions[1] != "RX_SERIAL" {
		t.Fatalf("second = %+v", second)
	}
}

func TestLEDStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"leds", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	status := data["leds"].(map[string]any)
	strip := status["strip"].(map[string]any)
	if strip["advanced_supported"] != true || strip["profile"] != float64(0) {
		t.Fatalf("strip = %+v", strip)
	}
	leds := strip["leds"].([]any)
	if len(leds) != 2 {
		t.Fatalf("leds = %+v", leds)
	}
	first := leds[0].(map[string]any)
	directions := first["directions"].([]any)
	overlays := first["overlays"].([]any)
	if first["cli_syntax"] != "1,2:ne:Ct:3" || directions[0] != "n" || directions[1] != "e" || overlays[0] != "t" {
		t.Fatalf("first = %+v", first)
	}
	colors := status["colors"].([]any)
	secondColor := colors[1].(map[string]any)
	if secondColor["hue"] != float64(120) || secondColor["sat"] != float64(255) {
		t.Fatalf("colors = %+v", colors)
	}
	values := status["values"].(map[string]any)
	if values["brightness"] != float64(50) || values["rainbow_freq"] != float64(120) {
		t.Fatalf("values = %+v", values)
	}
}

func TestPIDStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"pid", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	pid := data["pid"].(map[string]any)
	controller := pid["controller"].(map[string]any)
	if controller["name"] != "BETAFLIGHT" {
		t.Fatalf("controller = %+v", controller)
	}
	gains := pid["gains"].([]any)
	first := gains[0].(map[string]any)
	if first["name"] != "ROLL" || first["p"] != float64(45) || first["i"] != float64(80) || first["d"] != float64(30) {
		t.Fatalf("first gain = %+v", first)
	}
	advanced := pid["advanced"].(map[string]any)
	if advanced["anti_gravity_gain"] != float64(3500) || advanced["feedforward_pitch"] != float64(125) {
		t.Fatalf("advanced = %+v", advanced)
	}
	rateProfile := pid["rate_profile"].(map[string]any)
	if rateProfile["rates_type_name"] != "ACTUAL" {
		t.Fatalf("rate profile = %+v", rateProfile)
	}
}

func TestRatesStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"rates", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	rates := data["rates"].(map[string]any)
	rateProfile := rates["rate_profile"].(map[string]any)
	axes := rateProfile["axes"].([]any)
	roll := axes[0].(map[string]any)
	if roll["axis"] != "roll" || roll["rc_rate"] != float64(7) || roll["rate_limit_dps"] != float64(900) {
		t.Fatalf("roll = %+v", roll)
	}
	throttle := rateProfile["throttle"].(map[string]any)
	if throttle["limit_type_name"] != "SCALE" || throttle["hover_value"] != float64(0.45) {
		t.Fatalf("throttle = %+v", throttle)
	}
	tpa := rates["tpa"].(map[string]any)
	if tpa["mode"] != float64(2) || tpa["rate"] != float64(15) || tpa["breakpoint"] != float64(1350) {
		t.Fatalf("tpa = %+v", tpa)
	}
}

func TestFiltersStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"filters", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	filters := data["filters"].(map[string]any)
	advanced := filters["advanced_config"].(map[string]any)
	if advanced["pid_process_denom"] != float64(4) || advanced["motor_protocol_name"] != "DSHOT600" || advanced["gyro_check_overflow_name"] != "ALL" {
		t.Fatalf("advanced = %+v", advanced)
	}
	config := filters["filter_config"].(map[string]any)
	if config["gyro_lpf1_static_hz"] != float64(150) || config["gyro_lpf2_type_name"] != "PT2" {
		t.Fatalf("filter config = %+v", config)
	}
	dynamicNotch := config["dynamic_notch"].(map[string]any)
	if dynamicNotch["q"] != float64(300) || dynamicNotch["max_hz"] != float64(600) {
		t.Fatalf("dynamic notch = %+v", dynamicNotch)
	}
	rpmFilter := config["rpm_filter"].(map[string]any)
	weights := rpmFilter["weights"].([]any)
	if rpmFilter["harmonics"] != float64(3) || weights[2] != float64(60) {
		t.Fatalf("rpm filter = %+v", rpmFilter)
	}
}

func TestBatteryStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"battery", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	battery := data["battery"].(map[string]any)
	config := battery["config"].(map[string]any)
	if config["capacity_mah"] != float64(1300) || config["voltage_meter_source_name"] != "ADC" || config["warning_cell_voltage_v"] != float64(3.5) {
		t.Fatalf("config = %+v", config)
	}
	profile := battery["profile"].(map[string]any)
	if profile["force_cell_count"] != float64(4) || profile["full_cell_voltage_v"] != float64(4.2) {
		t.Fatalf("profile = %+v", profile)
	}
	state := battery["state"].(map[string]any)
	if state["state_name"] != "OK" || state["voltage_v"] != float64(15.99) {
		t.Fatalf("state = %+v", state)
	}
	voltageMeters := battery["voltage_meters"].([]any)
	firstVoltageMeter := voltageMeters[0].(map[string]any)
	if firstVoltageMeter["id_name"] != "BATTERY_1" || firstVoltageMeter["voltage_v"] != float64(16) {
		t.Fatalf("voltage meters = %+v", voltageMeters)
	}
	currentConfigs := battery["current_meter_configs"].([]any)
	firstCurrentConfig := currentConfigs[0].(map[string]any)
	if firstCurrentConfig["sensor_type_name"] != "ADC" || firstCurrentConfig["offset"] != float64(-10) {
		t.Fatalf("current configs = %+v", currentConfigs)
	}
}

func TestFailsafeStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"failsafe", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	failsafeStatus := data["failsafe"].(map[string]any)
	armingConfig := failsafeStatus["arming_config"].(map[string]any)
	if armingConfig["auto_disarm_delay_s"] != float64(5) || armingConfig["small_angle_degrees"] != float64(25) || armingConfig["gyro_cal_on_first_arm"] != true {
		t.Fatalf("arming config = %+v", armingConfig)
	}
	failsafe := failsafeStatus["failsafe_config"].(map[string]any)
	if failsafe["switch_mode_name"] != "STAGE2" || failsafe["procedure_name"] != "DROP" {
		t.Fatalf("failsafe = %+v", failsafe)
	}
	board := failsafeStatus["board_alignment"].(map[string]any)
	if board["roll_degrees"] != float64(-2) || board["yaw_degrees"] != float64(90) {
		t.Fatalf("board = %+v", board)
	}
	arming := failsafeStatus["arming"].(map[string]any)
	if arming["flag_count"] != float64(29) || arming["disabled"] != true {
		t.Fatalf("arming = %+v", arming)
	}
	activeNames := arming["active_names"].([]any)
	if activeNames[0] != "RXLOSS" || activeNames[len(activeNames)-1] != "CALIB" {
		t.Fatalf("active names = %+v", activeNames)
	}
}

func TestVTXTableListIncludesSummary(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "list"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	view := data["view"].(map[string]any)
	vtx := view["vtx"].(map[string]any)
	if vtx["bands"] != float64(1) || vtx["channels"] != float64(8) || vtx["power_levels"] != float64(2) {
		t.Fatalf("vtx = %+v", vtx)
	}
	bands := vtx["band_rows"].([]any)
	first := bands[0].(map[string]any)
	frequencies := first["frequencies_mhz"].([]any)
	if first["name"] != "RACEBAND" || first["factory"] != true || frequencies[0] != float64(5658) {
		t.Fatalf("band = %+v", first)
	}
	powerValues := vtx["power_values"].([]any)
	if len(powerValues) != 2 || powerValues[1] != float64(200) {
		t.Fatalf("power values = %+v", powerValues)
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
	env, err := runTestCommandWithInput(t, []string{"batch", "plan"}, "led 0 0,0::C:0\nservo 0 1000 2000 1500 100 -1\nadjrange 0 0 0 900 1300 12 0 0 0\nrxrange 0 1000 2000\nrxfail 2 s 1100\nbattery_profile 1\n", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	lines := data["cli_lines"].([]any)
	if len(lines) != 6 {
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
		{[]string{"battery", "list"}, "settings"},
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

func TestRebootRefusalDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"reboot", "firmware"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called for refused reboot")
	}
}

func TestRebootFirmwareWithFakeFC(t *testing.T) {
	var operation connection.OperationClass
	env, err := runTestCommand(t, []string{"reboot", "firmware", "--yes"}, func(ctx context.Context, cfg connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		operation = op
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
	if operation != connection.Dangerous {
		t.Fatalf("operation = %v, want Dangerous", operation)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	reboot := data["reboot"].(map[string]any)
	if reboot["mode_name"] != "firmware" || reboot["acknowledged"] != true {
		t.Fatalf("reboot = %+v", reboot)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "firmware_reboot" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRebootMSCWithFakeFCReportsReady(t *testing.T) {
	env, err := runTestCommand(t, []string{"reboot", "msc", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	reboot := data["reboot"].(map[string]any)
	if reboot["mode_name"] != "msc" || reboot["msc_ready"] != true {
		t.Fatalf("reboot = %+v", reboot)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "msc_reboot" {
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
