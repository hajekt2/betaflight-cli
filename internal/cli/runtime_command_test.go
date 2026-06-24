package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func TestTelemetrySnapshotWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"telemetry", "snapshot"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data, ok := env.Data.(map[string]any)
	telemetry, _ := data["telemetry"].(map[string]any)
	if !ok || telemetry["attitude"] == nil || telemetry["attitude_quaternion"] == nil || telemetry["battery"] == nil || telemetry["rc"] == nil {
		t.Fatalf("unexpected telemetry data: %+v", env.Data)
	}
	quaternion := telemetry["attitude_quaternion"].(map[string]any)
	if quaternion["w"] != float64(1) || quaternion["x"] != float64(0) || quaternion["y"] != float64(0) || quaternion["z"] != float64(0) {
		t.Fatalf("attitude quaternion = %+v", quaternion)
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
	telemetry := data["telemetry"].(map[string]any)
	sources, ok := telemetry["sources"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected telemetry sources: %+v", data["sources"])
	}
	if sources["status"] == nil || sources["attitude"] == nil || sources["attitude_quaternion"] == nil || sources["battery"] == nil || sources["rc"] == nil {
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

func TestDebugSetAccelerometerTrimRequiresValidInt16(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"debug", "set-accelerometer-trim", "40000", "0", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after trim validation failure")
	}
}

func TestDebugSetAccelerometerTrimRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"debug", "set-accelerometer-trim", "--", "-12", "34"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after trim confirmation failure")
	}
}

func TestDebugSetAccelerometerTrimJSONRequiresYesDoesNotConnect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trim.json")
	if err := os.WriteFile(path, []byte(`{"accelerometer_trim":{"pitch":-12,"roll":34}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	called := false
	env, err := runTestCommand(t, []string{"debug", "set-accelerometer-trim-json", path}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after trim confirmation failure")
	}
}

func TestDebugSetAccelerometerTrimWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"--yes", "debug", "set-accelerometer-trim", "--", "-12", "34"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["accelerometer_trim"].(map[string]any)
	trim := result["trim"].(map[string]any)
	if trim["pitch"] != float64(-12) || trim["roll"] != float64(34) || result["msp_name"] != "MSP_SET_ACC_TRIM" {
		t.Fatalf("accelerometer trim = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "accelerometer_trim" || env.SideEffects[0].Command != "MSP_SET_ACC_TRIM" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestDebugSetAccelerometerTrimJSONWithFakeFC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trim.json")
	if err := os.WriteFile(path, []byte(`{"trim":{"pitch":-7,"roll":11}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	env, err := runTestCommand(t, []string{"debug", "set-accelerometer-trim-json", path, "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["accelerometer_trim"].(map[string]any)
	trim := result["trim"].(map[string]any)
	if trim["pitch"] != float64(-7) || trim["roll"] != float64(11) || result["msp_name"] != "MSP_SET_ACC_TRIM" {
		t.Fatalf("accelerometer trim = %+v", result)
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

func TestRTCSetRequiresTimestampOrNow(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"rtc", "set", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after rtc set validation failure")
	}
}

func TestRTCSetRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"rtc", "set", "--timestamp", "2026-06-23T12:34:56.789Z"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after rtc set confirmation failure")
	}
}

func TestRTCSetJSONRequiresYesDoesNotConnect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rtc.json")
	if err := os.WriteFile(path, []byte(`{"timestamp_utc":"2026-06-23T12:34:56.789Z"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	called := false
	env, err := runTestCommand(t, []string{"rtc", "set-json", path}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after rtc set confirmation failure")
	}
}

func TestRTCSetJSONRequiresExactlyOneTimeSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rtc.json")
	if err := os.WriteFile(path, []byte(`{"timestamp_utc":"2026-06-23T12:34:56.789Z","now":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	called := false
	env, err := runTestCommand(t, []string{"rtc", "set-json", path, "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after rtc set validation failure")
	}
}

func TestRTCSetWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"rtc", "set", "--timestamp", "2026-06-23T12:34:56.789Z", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	rtc := data["rtc"].(map[string]any)
	if rtc["timestamp_utc"] != "2026-06-23T12:34:56.789Z" || rtc["msp_name"] != "MSP_SET_RTC" || rtc["acknowledged"] != true {
		t.Fatalf("rtc = %+v", rtc)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "rtc_set" || env.SideEffects[0].Command != "MSP_SET_RTC" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRTCSetJSONWithFakeFC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rtc.json")
	if err := os.WriteFile(path, []byte(`{"rtc":{"iso_utc":"2026-06-23T12:34:56.789Z"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	env, err := runTestCommand(t, []string{"rtc", "set-json", path, "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	rtc := data["rtc"].(map[string]any)
	if rtc["timestamp_utc"] != "2026-06-23T12:34:56.789Z" || rtc["msp_name"] != "MSP_SET_RTC" || rtc["acknowledged"] != true {
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
