package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	if called {
		t.Fatal("connector was called for offline blackbox inspect")
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
		"I\x00P\x00E",
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
