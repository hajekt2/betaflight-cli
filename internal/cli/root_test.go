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

	"github.com/spf13/cobra"

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
		{"help", cliReadOnly},
		{"resource", cliReadOnly},
		{"resource show", cliReadOnly},
		{"resource list", cliReadOnly},
		{"resource serialrx 1 A06", cliWrite},
		{"set gyro_lpf1_static_hz = 0", cliWrite},
		{"beeper 1", cliDangerous},
		{"save", cliDangerous},
		{"defaults", cliDangerous},
		{"motor 0 1000", cliDangerous},
		{"    ", cliReadOnly},
	}
	for _, tt := range tests {
		if got := classifyCLI(tt.command); got != tt.want {
			t.Fatalf("classifyCLI(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

func TestIsKnownCLICommand(t *testing.T) {
	tests := map[string]bool{
		"set foo = 1":        true,
		"resource serialrx 1": true,
		"help":               true,
		"impossible line":    false,
		"":                   false,
		"   ":                false,
	}
	for line, want := range tests {
		if got := isKnownCLICommand(line); got != want {
			t.Fatalf("isKnownCLICommand(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestIsBatchAllowed(t *testing.T) {
	tests := map[string]bool{
		"set foo = 1":        true,
		"serial 0 1 1":       true,
		"resource serialrx 1": true,
		"save":               false,
		"diff all":           false,
		"reboot":             false,
		"feature GPS":        true,
		"beeper 1":           true,
		"map 1 2 3":          true,
		"timer 2":            true,
		"dma 1":              true,
	}
	for line, want := range tests {
		if got := isBatchAllowed(line); got != want {
			t.Fatalf("isBatchAllowed(%q) = %v, want %v", line, got, want)
		}
	}
}

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
	env, err := runTestCommand(t, []string{"cli", "exec", "set gyro_lpf1_static_hz = 0", "--yes"}, nil)
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

	env, err = runTestCommand(t, []string{"cli", "exec", "mystery command", "--yes"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("expected OK with --yes, got errors: %+v", env.Errors)
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

func TestMSPRequestWriteLikeRequiresYes(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"msp", "request", "11"}, "", func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("msp request was sent without --yes")
	}

	_, err = runTestCommandWithInput(t, []string{"msp", "request", "11", "--yes", "--payload-hex", ""}, "", nil)
	if err != nil {
		// connection path is backed by fake serial transport in tests, so protocol-level failures are expected.
	}
}

func TestMSPRequestInvalidCodeReturnsValidationError(t *testing.T) {
	// invalid code that cannot be resolved must fail before attempting transport
	called := false
	env, err := runTestCommand(t, []string{"msp", "request", "not-a-code"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) == 0 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope for invalid code: %+v", env.Errors)
	}
	if called {
		t.Fatal("msp request attempted transport for invalid code")
	}
}

func TestMSPRequestInvalidPayloadHexFails(t *testing.T) {
	// malformed hex should fail with explicit payload error
	called := false
	env, err := runTestCommandWithInput(t, []string{"msp", "request", "MSP_NAME", "--yes", "--payload-hex", "Z1"}, "", func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) == 0 || !strings.Contains(strings.ToLower(env.Errors[0].Message), "encoding/hex") {
		t.Fatalf("unexpected envelope for invalid payload hex: %+v", env.Errors)
	}
	if called {
		t.Fatal("msp request attempted transport for invalid payload")
	}
}

func TestMSPRequestReadLikeDoesNotRequireYes(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"msp", "request", "MSP_API_VERSION"}, "", func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	for _, e := range env.Errors {
		if e.Code == "confirmation_required" {
			t.Fatalf("unexpected confirmation_required for read-like MSP request: %+v", env.Errors)
		}
	}
	if !called {
		t.Fatal("msp request did not attempt connection")
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
	configurationPlan := byCommand["betaflight-cli configuration plan"]
	if configurationPlan["operation"] != "offline" || configurationPlan["requires_connection"] != false || configurationPlan["confirmation"] != "none" || configurationPlan["runnable"] != true {
		t.Fatalf("configuration plan capability = %+v", configurationPlan)
	}
	configurationApply := byCommand["betaflight-cli configuration apply"]
	if configurationApply["operation"] != "write" || configurationApply["requires_connection"] != true || configurationApply["confirmation"] != "--yes" || configurationApply["runnable"] != true {
		t.Fatalf("configuration apply capability = %+v", configurationApply)
	}
	settingsDiff := byCommand["betaflight-cli settings diff"]
	if settingsDiff["operation"] != "read_only" || settingsDiff["requires_connection"] != true || settingsDiff["confirmation"] != "none" || settingsDiff["runnable"] != true {
		t.Fatalf("settings diff capability = %+v", settingsDiff)
	}
	featureMask := byCommand["betaflight-cli features set-mask"]
	if featureMask["operation"] != "write" || featureMask["confirmation"] != "--yes" || featureMask["requires_connection"] != true || featureMask["output_root"] != "feature_mask" || featureMask["runnable"] != true {
		t.Fatalf("feature mask capability = %+v", featureMask)
	}
	beeperEnable := byCommand["betaflight-cli beeper enable"]
	if beeperEnable["operation"] != "plan_or_write" || beeperEnable["requires_connection"] != true || beeperEnable["confirmation"] != "--yes with --apply or --save" || beeperEnable["runnable"] != true {
		t.Fatalf("beeper enable capability = %+v", beeperEnable)
	}
	beeperDisable := byCommand["betaflight-cli beeper disable"]
	if beeperDisable["operation"] != "plan_or_write" || beeperDisable["requires_connection"] != true || beeperDisable["confirmation"] != "--yes with --apply or --save" || beeperDisable["runnable"] != true {
		t.Fatalf("beeper disable capability = %+v", beeperDisable)
	}
	beeperConfig := byCommand["betaflight-cli beeper set-config"]
	if beeperConfig["operation"] != "write" || beeperConfig["confirmation"] != "--yes" || beeperConfig["requires_connection"] != true || beeperConfig["output_root"] != "beeper_config" || beeperConfig["runnable"] != true {
		t.Fatalf("beeper config capability = %+v", beeperConfig)
	}
	cliInteractive := byCommand["betaflight-cli cli interactive"]
	if cliInteractive["operation"] != "dangerous" || cliInteractive["confirmation"] != "--yes" || cliInteractive["requires_connection"] != true || cliInteractive["runnable"] != true {
		t.Fatalf("cli interactive capability = %+v", cliInteractive)
	}
	sensorCalibration := byCommand["betaflight-cli sensors calibrate-accelerometer"]
	if sensorCalibration["operation"] != "dangerous" || sensorCalibration["confirmation"] != "--yes" || sensorCalibration["requires_connection"] != true || sensorCalibration["output_root"] != "sensor_calibration" || sensorCalibration["runnable"] != true {
		t.Fatalf("sensor calibration capability = %+v", sensorCalibration)
	}
	rtcSet := byCommand["betaflight-cli rtc set"]
	if rtcSet["operation"] != "write" || rtcSet["confirmation"] != "--yes" || rtcSet["requires_connection"] != true || rtcSet["output_root"] != "rtc" || rtcSet["runnable"] != true {
		t.Fatalf("rtc set capability = %+v", rtcSet)
	}
	accTrim := byCommand["betaflight-cli debug set-accelerometer-trim"]
	if accTrim["operation"] != "write" || accTrim["confirmation"] != "--yes" || accTrim["requires_connection"] != true || accTrim["output_root"] != "accelerometer_trim" || accTrim["runnable"] != true {
		t.Fatalf("accelerometer trim capability = %+v", accTrim)
	}
	sensorConfig := byCommand["betaflight-cli sensors set-config"]
	if sensorConfig["operation"] != "write" || sensorConfig["confirmation"] != "--yes" || sensorConfig["requires_connection"] != true || sensorConfig["output_root"] != "sensor_config" || sensorConfig["runnable"] != true {
		t.Fatalf("sensor config capability = %+v", sensorConfig)
	}
	sensorAlignment := byCommand["betaflight-cli sensors set-alignment"]
	if sensorAlignment["operation"] != "write" || sensorAlignment["confirmation"] != "--yes" || sensorAlignment["requires_connection"] != true || sensorAlignment["output_root"] != "sensor_alignment" || sensorAlignment["runnable"] != true {
		t.Fatalf("sensor alignment capability = %+v", sensorAlignment)
	}
	compassConfig := byCommand["betaflight-cli sensors set-compass-declination"]
	if compassConfig["operation"] != "write" || compassConfig["confirmation"] != "--yes" || compassConfig["requires_connection"] != true || compassConfig["output_root"] != "compass_config" || compassConfig["runnable"] != true {
		t.Fatalf("compass config capability = %+v", compassConfig)
	}
	profileCopy := byCommand["betaflight-cli profiles copy"]
	if profileCopy["operation"] != "write" || profileCopy["confirmation"] != "--yes" || profileCopy["requires_connection"] != true || profileCopy["output_root"] != "profile_copy" || profileCopy["runnable"] != true {
		t.Fatalf("profile copy capability = %+v", profileCopy)
	}
	textSet := byCommand["betaflight-cli text set"]
	if textSet["operation"] != "write" || textSet["confirmation"] != "--yes" || textSet["requires_connection"] != true || textSet["output_root"] != "text" || textSet["runnable"] != true {
		t.Fatalf("text set capability = %+v", textSet)
	}
	boardAlignment := byCommand["betaflight-cli failsafe set-board-alignment"]
	if boardAlignment["operation"] != "write" || boardAlignment["confirmation"] != "--yes" || boardAlignment["requires_connection"] != true || boardAlignment["output_root"] != "board_alignment" || boardAlignment["runnable"] != true {
		t.Fatalf("board alignment capability = %+v", boardAlignment)
	}
	receiverConfig := byCommand["betaflight-cli receiver set-config-json"]
	if receiverConfig["operation"] != "write" || receiverConfig["confirmation"] != "--yes" || receiverConfig["requires_connection"] != true || receiverConfig["output_root"] != "receiver_config" || receiverConfig["runnable"] != true {
		t.Fatalf("receiver config capability = %+v", receiverConfig)
	}
	rssiChannel := byCommand["betaflight-cli receiver set-rssi-channel"]
	if rssiChannel["operation"] != "write" || rssiChannel["confirmation"] != "--yes" || rssiChannel["requires_connection"] != true || rssiChannel["output_root"] != "rssi_channel" || rssiChannel["runnable"] != true {
		t.Fatalf("rssi channel capability = %+v", rssiChannel)
	}
	rxFail := byCommand["betaflight-cli receiver set-rxfail"]
	if rxFail["operation"] != "write" || rxFail["confirmation"] != "--yes" || rxFail["requires_connection"] != true || rxFail["output_root"] != "rx_fail" || rxFail["runnable"] != true {
		t.Fatalf("rx fail capability = %+v", rxFail)
	}
	rcMap := byCommand["betaflight-cli receiver set-map"]
	if rcMap["operation"] != "write" || rcMap["confirmation"] != "--yes" || rcMap["requires_connection"] != true || rcMap["output_root"] != "rc_map" || rcMap["runnable"] != true {
		t.Fatalf("rc map capability = %+v", rcMap)
	}
	rcDeadband := byCommand["betaflight-cli receiver set-deadband"]
	if rcDeadband["operation"] != "write" || rcDeadband["confirmation"] != "--yes" || rcDeadband["requires_connection"] != true || rcDeadband["output_root"] != "rc_deadband" || rcDeadband["runnable"] != true {
		t.Fatalf("rc deadband capability = %+v", rcDeadband)
	}
	gpsConfig := byCommand["betaflight-cli gps set-config"]
	if gpsConfig["operation"] != "write" || gpsConfig["confirmation"] != "--yes" || gpsConfig["requires_connection"] != true || gpsConfig["output_root"] != "gps_config" || gpsConfig["runnable"] != true {
		t.Fatalf("gps config capability = %+v", gpsConfig)
	}
	gpsRescue := byCommand["betaflight-cli gps set-rescue"]
	if gpsRescue["operation"] != "write" || gpsRescue["confirmation"] != "--yes" || gpsRescue["requires_connection"] != true || gpsRescue["output_root"] != "gps_rescue" || gpsRescue["runnable"] != true {
		t.Fatalf("gps rescue capability = %+v", gpsRescue)
	}
	gpsRescuePIDs := byCommand["betaflight-cli gps set-rescue-pids"]
	if gpsRescuePIDs["operation"] != "write" || gpsRescuePIDs["confirmation"] != "--yes" || gpsRescuePIDs["requires_connection"] != true || gpsRescuePIDs["output_root"] != "gps_rescue_pids" || gpsRescuePIDs["runnable"] != true {
		t.Fatalf("gps rescue pids capability = %+v", gpsRescuePIDs)
	}
	failsafeConfig := byCommand["betaflight-cli failsafe set-config-json"]
	if failsafeConfig["operation"] != "write" || failsafeConfig["confirmation"] != "--yes" || failsafeConfig["requires_connection"] != true || failsafeConfig["output_root"] != "failsafe_config" || failsafeConfig["runnable"] != true {
		t.Fatalf("failsafe config capability = %+v", failsafeConfig)
	}
	pidGains := byCommand["betaflight-cli pid set-gains-json"]
	if pidGains["operation"] != "write" || pidGains["confirmation"] != "--yes" || pidGains["requires_connection"] != true || pidGains["output_root"] != "pid_gains" || pidGains["runnable"] != true {
		t.Fatalf("pid gains capability = %+v", pidGains)
	}
	pidAdvanced := byCommand["betaflight-cli pid set-advanced-json"]
	if pidAdvanced["operation"] != "write" || pidAdvanced["confirmation"] != "--yes" || pidAdvanced["requires_connection"] != true || pidAdvanced["output_root"] != "pid_advanced" || pidAdvanced["runnable"] != true {
		t.Fatalf("pid advanced capability = %+v", pidAdvanced)
	}
	rateProfile := byCommand["betaflight-cli rates set-profile-json"]
	if rateProfile["operation"] != "write" || rateProfile["confirmation"] != "--yes" || rateProfile["requires_connection"] != true || rateProfile["output_root"] != "rate_profile" || rateProfile["runnable"] != true {
		t.Fatalf("rate profile capability = %+v", rateProfile)
	}
	advancedConfig := byCommand["betaflight-cli filters set-advanced-json"]
	if advancedConfig["operation"] != "write" || advancedConfig["confirmation"] != "--yes" || advancedConfig["requires_connection"] != true || advancedConfig["output_root"] != "advanced_config" || advancedConfig["runnable"] != true {
		t.Fatalf("advanced config capability = %+v", advancedConfig)
	}
	filterConfig := byCommand["betaflight-cli filters set-filter-json"]
	if filterConfig["operation"] != "write" || filterConfig["confirmation"] != "--yes" || filterConfig["requires_connection"] != true || filterConfig["output_root"] != "filter_config" || filterConfig["runnable"] != true {
		t.Fatalf("filter config capability = %+v", filterConfig)
	}
	blackboxConfig := byCommand["betaflight-cli blackbox set-config-json"]
	if blackboxConfig["operation"] != "write" || blackboxConfig["confirmation"] != "--yes" || blackboxConfig["requires_connection"] != true || blackboxConfig["output_root"] != "blackbox_config" || blackboxConfig["runnable"] != true {
		t.Fatalf("blackbox config capability = %+v", blackboxConfig)
	}
	mixerConfig := byCommand["betaflight-cli mixer set-config-json"]
	if mixerConfig["operation"] != "write" || mixerConfig["confirmation"] != "--yes" || mixerConfig["requires_connection"] != true || mixerConfig["output_root"] != "mixer_config" || mixerConfig["runnable"] != true {
		t.Fatalf("mixer config capability = %+v", mixerConfig)
	}
	motorConfig := byCommand["betaflight-cli motors set-config"]
	if motorConfig["operation"] != "write" || motorConfig["confirmation"] != "--yes" || motorConfig["requires_connection"] != true || motorConfig["output_root"] != "motor_config" || motorConfig["runnable"] != true {
		t.Fatalf("motor config capability = %+v", motorConfig)
	}
	motor3DConfig := byCommand["betaflight-cli motors set-3d-config"]
	if motor3DConfig["operation"] != "write" || motor3DConfig["confirmation"] != "--yes" || motor3DConfig["requires_connection"] != true || motor3DConfig["output_root"] != "motor_3d_config" || motor3DConfig["runnable"] != true {
		t.Fatalf("motor 3d config capability = %+v", motor3DConfig)
	}
	serialConfig := byCommand["betaflight-cli serial apply-config-json"]
	if serialConfig["operation"] != "write" || serialConfig["confirmation"] != "--yes" || serialConfig["requires_connection"] != true || serialConfig["output_root"] != "serial_config" || serialConfig["runnable"] != true {
		t.Fatalf("serial config capability = %+v", serialConfig)
	}
	modeRange := byCommand["betaflight-cli modes set-range"]
	if modeRange["operation"] != "write" || modeRange["confirmation"] != "--yes" || modeRange["requires_connection"] != true || modeRange["output_root"] != "mode_range" || modeRange["runnable"] != true {
		t.Fatalf("mode range capability = %+v", modeRange)
	}
	ledValues := byCommand["betaflight-cli leds set-values"]
	if ledValues["operation"] != "write" || ledValues["confirmation"] != "--yes" || ledValues["requires_connection"] != true || ledValues["output_root"] != "led_values" || ledValues["runnable"] != true {
		t.Fatalf("led values capability = %+v", ledValues)
	}
	servoConfig := byCommand["betaflight-cli servos set-config"]
	if servoConfig["operation"] != "write" || servoConfig["confirmation"] != "--yes" || servoConfig["requires_connection"] != true || servoConfig["output_root"] != "servo_config" || servoConfig["runnable"] != true {
		t.Fatalf("servo config capability = %+v", servoConfig)
	}
	servoMixRule := byCommand["betaflight-cli servos set-mix-rule"]
	if servoMixRule["operation"] != "write" || servoMixRule["confirmation"] != "--yes" || servoMixRule["requires_connection"] != true || servoMixRule["output_root"] != "servo_mix_rule" || servoMixRule["runnable"] != true {
		t.Fatalf("servo mix rule capability = %+v", servoMixRule)
	}
	adjustmentRange := byCommand["betaflight-cli adjustments set-range"]
	if adjustmentRange["operation"] != "write" || adjustmentRange["confirmation"] != "--yes" || adjustmentRange["requires_connection"] != true || adjustmentRange["output_root"] != "adjustment_range" || adjustmentRange["runnable"] != true {
		t.Fatalf("adjustment range capability = %+v", adjustmentRange)
	}
	batteryConfig := byCommand["betaflight-cli battery set-config-json"]
	if batteryConfig["operation"] != "write" || batteryConfig["confirmation"] != "--yes" || batteryConfig["requires_connection"] != true || batteryConfig["output_root"] != "battery_config" || batteryConfig["runnable"] != true {
		t.Fatalf("battery config capability = %+v", batteryConfig)
	}
	voltageMeter := byCommand["betaflight-cli battery set-voltage-meter"]
	if voltageMeter["operation"] != "write" || voltageMeter["confirmation"] != "--yes" || voltageMeter["requires_connection"] != true || voltageMeter["output_root"] != "voltage_meter_config" || voltageMeter["runnable"] != true {
		t.Fatalf("voltage meter capability = %+v", voltageMeter)
	}
	currentMeter := byCommand["betaflight-cli battery set-current-meter"]
	if currentMeter["operation"] != "write" || currentMeter["confirmation"] != "--yes" || currentMeter["requires_connection"] != true || currentMeter["output_root"] != "current_meter_config" || currentMeter["runnable"] != true {
		t.Fatalf("current meter capability = %+v", currentMeter)
	}
	cliExec := byCommand["betaflight-cli cli exec"]
	if cliExec["operation"] != "read_only_or_write_or_dangerous" || cliExec["confirmation"] != "--yes for writes and dangerous CLI lines" || cliExec["requires_connection"] != true || cliExec["runnable"] != true {
		t.Fatalf("cli exec capability = %+v", cliExec)
	}
	vtxList := byCommand["betaflight-cli vtx list"]
	if vtxList["operation"] != "read_only" || vtxList["confirmation"] != "none" || vtxList["requires_connection"] != true || vtxList["runnable"] != true {
		t.Fatalf("vtx list capability = %+v", vtxList)
	}
	mspRequest := byCommand["betaflight-cli msp request"]
	if mspRequest["operation"] != "read_only_or_write_or_dangerous" || mspRequest["requires_connection"] != true || mspRequest["confirmation"] != "read-only unless --code implies write; write commands require --yes" || mspRequest["runnable"] != true {
		t.Fatalf("msp request capability = %+v", mspRequest)
	}
	mspList := byCommand["betaflight-cli msp list"]
	if mspList["operation"] != "offline" || mspList["requires_connection"] != false || mspList["confirmation"] != "none" || mspList["runnable"] != true {
		t.Fatalf("msp list capability = %+v", mspList)
	}
	mspMetadata := byCommand["betaflight-cli msp metadata"]
	if mspMetadata["operation"] != "offline" || mspMetadata["requires_connection"] != false || mspMetadata["confirmation"] != "none" || mspMetadata["runnable"] != true {
		t.Fatalf("msp metadata capability = %+v", mspMetadata)
	}
	schema := byCommand["betaflight-cli schema"]
	if schema["operation"] != "offline" || schema["requires_connection"] != false || schema["confirmation"] != "none" || schema["runnable"] != true {
		t.Fatalf("schema capability = %+v", schema)
	}
	capabilitiesCoverage := byCommand["betaflight-cli capabilities coverage"]
	if capabilitiesCoverage["operation"] != "offline" || capabilitiesCoverage["requires_connection"] != false || capabilitiesCoverage["confirmation"] != "none" || capabilitiesCoverage["runnable"] != true {
		t.Fatalf("capabilities coverage capability = %+v", capabilitiesCoverage)
	}
	transponderSetProvider := byCommand["betaflight-cli transponder set-provider"]
	if transponderSetProvider["operation"] != "plan_or_write" || transponderSetProvider["requires_connection"] != true || transponderSetProvider["confirmation"] != "--yes with --apply or --save" || transponderSetProvider["runnable"] != true {
		t.Fatalf("transponder set-provider capability = %+v", transponderSetProvider)
	}
	transponderSetData := byCommand["betaflight-cli transponder set-data"]
	if transponderSetData["operation"] != "plan_or_write" || transponderSetData["requires_connection"] != true || transponderSetData["confirmation"] != "--yes with --apply or --save" || transponderSetData["runnable"] != true {
		t.Fatalf("transponder set-data capability = %+v", transponderSetData)
	}
	transponderConfig := byCommand["betaflight-cli transponder set-config"]
	if transponderConfig["operation"] != "write" || transponderConfig["confirmation"] != "--yes" || transponderConfig["requires_connection"] != true || transponderConfig["output_root"] != "transponder_config" || transponderConfig["runnable"] != true {
		t.Fatalf("transponder config capability = %+v", transponderConfig)
	}
	vtxConfig := byCommand["betaflight-cli vtx set-config"]
	if vtxConfig["operation"] != "write" || vtxConfig["confirmation"] != "--yes" || vtxConfig["requires_connection"] != true || vtxConfig["output_root"] != "vtx_config" || vtxConfig["runnable"] != true {
		t.Fatalf("vtx config capability = %+v", vtxConfig)
	}
	osdCanvas := byCommand["betaflight-cli osd set-canvas"]
	if osdCanvas["operation"] != "write" || osdCanvas["confirmation"] != "--yes" || osdCanvas["requires_connection"] != true || osdCanvas["output_root"] != "osd_canvas" || osdCanvas["runnable"] != true {
		t.Fatalf("osd canvas capability = %+v", osdCanvas)
	}
	osdPosition := byCommand["betaflight-cli osd set-position"]
	if osdPosition["operation"] != "write" || osdPosition["confirmation"] != "--yes" || osdPosition["requires_connection"] != true || osdPosition["output_root"] != "osd_position" || osdPosition["runnable"] != true {
		t.Fatalf("osd position capability = %+v", osdPosition)
	}
	osdStat := byCommand["betaflight-cli osd set-stat"]
	if osdStat["operation"] != "write" || osdStat["confirmation"] != "--yes" || osdStat["requires_connection"] != true || osdStat["output_root"] != "osd_stat" || osdStat["runnable"] != true {
		t.Fatalf("osd stat capability = %+v", osdStat)
	}
	osdTimer := byCommand["betaflight-cli osd set-timer"]
	if osdTimer["operation"] != "write" || osdTimer["confirmation"] != "--yes" || osdTimer["requires_connection"] != true || osdTimer["output_root"] != "osd_timer" || osdTimer["runnable"] != true {
		t.Fatalf("osd timer capability = %+v", osdTimer)
	}
	vtxTableBand := byCommand["betaflight-cli vtxtable set-band"]
	if vtxTableBand["operation"] != "write" || vtxTableBand["confirmation"] != "--yes" || vtxTableBand["requires_connection"] != true || vtxTableBand["output_root"] != "vtxtable_band" || vtxTableBand["runnable"] != true {
		t.Fatalf("vtxtable band capability = %+v", vtxTableBand)
	}
	vtxTablePower := byCommand["betaflight-cli vtxtable set-power"]
	if vtxTablePower["operation"] != "write" || vtxTablePower["confirmation"] != "--yes" || vtxTablePower["requires_connection"] != true || vtxTablePower["output_root"] != "vtxtable_power" || vtxTablePower["runnable"] != true {
		t.Fatalf("vtxtable power capability = %+v", vtxTablePower)
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

func TestSchemaCommandDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"schema"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatalf("schema command should be offline and never connect")
	}
	data := env.Data.(map[string]any)
	if data["command"] != "schema" {
		t.Fatalf("command = %v", data["command"])
	}
	schemaVersion := data["schema_version"].(map[string]any)
	if schemaVersion["envelope"] != output.SchemaVersion {
		t.Fatalf("schema_version = %+v", schemaVersion)
	}
	contract := data["command_contracts"].(map[string]any)
	if contract["total_commands"] == nil {
		t.Fatalf("command_contracts = %+v", contract)
	}
	if contract["runnable_commands"] == nil || contract["requires_connection_commands"] == nil {
		t.Fatalf("command_contracts missing counts = %+v", contract)
	}
	operations := contract["operations"].([]any)
	if len(operations) < 6 {
		t.Fatalf("operations = %+v", operations)
	}
	operationCounts := contract["operation_counts"].(map[string]any)
	if len(operationCounts) == 0 {
		t.Fatalf("operation_counts = %+v", contract)
	}
	for key, total := range operationCounts {
		if total == nil || total.(float64) <= 0 {
			t.Fatalf("operation_counts[%s]=%v", key, total)
		}
	}
	outputRoots := data["output_roots"].([]any)
	if len(outputRoots) == 0 {
		t.Fatalf("output_roots = %+v", outputRoots)
	}
	caps := data["capabilities"].(map[string]any)
	coverage := caps["coverage"].(map[string]any)
	if coverage["implemented_domains"] == nil || coverage["partial_domains"] == nil || coverage["domain_count"] == nil {
		t.Fatalf("capabilities.coverage = %+v", coverage)
	}
	if gaps, ok := coverage["next_gaps"].([]any); !ok || len(gaps) == 0 {
		t.Fatalf("capabilities.coverage.next_gaps = %+v", coverage["next_gaps"])
	}
}

func TestMSPListDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"msp", "list"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if called {
		t.Fatalf("msp list should not connect")
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("env.Data type = %T", env.Data)
	}
	if data["count"] == nil {
		t.Fatalf("msp list payload = %+v", data)
	}
}

func TestMSPMetadataDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"msp", "metadata", "MSP_NAME"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if called {
		t.Fatalf("msp metadata should not connect")
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("env.Data type = %T", env.Data)
	}
	if data["code"] == nil || data["code_name"] != "MSP_NAME" {
		t.Fatalf("msp metadata payload = %+v", data)
	}
}

func TestFirmwareFlashPlanModeWorksOffline(t *testing.T) {
	tmp := t.TempDir()
	image := filepath.Join(tmp, "firmware.bin")
	if err := os.WriteFile(image, []byte{0xaa, 0xbb}, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	env, err := runTestCommand(t, []string{
		"firmware", "flash", "--image", image, "--tool", "dfu-util", "--tool-arg", "-a", "0", "--tool-arg", "-s", "0x08000000:leave",
	}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	flash := data["firmware_flash"].(map[string]any)
	if flash["executed"].(bool) {
		t.Fatalf("plan mode must not execute: %+v", flash)
	}
	if flash["action"].(string) != "plan" {
		t.Fatalf("plan action = %v", flash["action"])
	}
}

func TestFirmwareFlashExecuteRequiresYes(t *testing.T) {
	tmp := t.TempDir()
	image := filepath.Join(tmp, "firmware.bin")
	if err := os.WriteFile(image, []byte{0x11, 0x22}, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	env, err := runTestCommand(t, []string{
		"firmware", "flash", "--image", image, "--tool", "dfu-util", "--execute",
	}, nil)
	if err == nil {
		t.Fatalf("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestFirmwareFlashPlanModeRequiresImage(t *testing.T) {
	env, err := runTestCommand(t, []string{"firmware", "flash", "--tool", "dfu-util"}, nil)
	if err == nil {
		t.Fatalf("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
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
	data := env.Data.(map[string]any)
	capabilities := data["capabilities"].(map[string]any)
	safety := capabilities["safety"].(map[string]any)
	if safety["auto_port_writes_require_opt_in"].(bool) {
		t.Fatalf("safety auto_port_writes_require_opt_in should be false")
	}
	coverage := data["coverage"].(map[string]any)
	summary := coverage["summary"].(map[string]any)
	if summary["domain_count"].(float64) < 15 || summary["implemented_count"].(float64) < 10 {
		t.Fatalf("summary = %+v", summary)
	}
	domains := coverage["domains"].([]any)
	if len(domains) != int(summary["domain_count"].(float64)) {
		t.Fatalf("coverage domain_count mismatch: got=%d want=%v", len(domains), summary["domain_count"])
	}
	domainCounts := map[string]int{}
	byDomain := map[string]map[string]any{}
	for _, item := range domains {
		domain := item.(map[string]any)
		name := domain["domain"].(string)
		domainCounts[name]++
		byDomain[domain["domain"].(string)] = domain
	}
	for name, count := range domainCounts {
		if count != 1 {
			t.Fatalf("coverage domain %q has %d entries, expected 1", name, count)
		}
	}
	backup := byDomain["configuration-backup"]
	if backup["status"] != "implemented" || len(backup["read_commands"].([]any)) == 0 {
		t.Fatalf("backup domain = %+v", backup)
	}
	maintenance := byDomain["firmware-maintenance"]
	if maintenance["status"] != "implemented" || len(maintenance["dangerous_commands"].([]any)) == 0 {
		t.Fatalf("maintenance domain = %+v", maintenance)
	}
	flashing := byDomain["firmware-flashing"]
	if flashing["status"] != "implemented" || len(flashing["dangerous_commands"].([]any)) == 0 {
		t.Fatalf("firmware-flashing domain = %+v", flashing)
	}
	motors := byDomain["motors-servos-mixer"]
	if motors["status"] != "implemented" {
		t.Fatalf("motors domain = %+v", motors)
	}
	settings := byDomain["settings"]
	if settings["status"] != "implemented" || len(settings["read_commands"].([]any)) == 0 || len(settings["write_commands"].([]any)) == 0 {
		t.Fatalf("settings domain = %+v", settings)
	}
	gaps := coverage["next_gaps"].([]any)
	for _, item := range gaps {
		gap := item.(map[string]any)
		if gap["domain"] == "blackbox-decoding" {
			t.Fatalf("unexpected blackbox-decoding gap remains: %+v", gap)
		}
		if gap["domain"] == "motor-testing" {
			t.Fatalf("unexpected motor-testing gap remains: %+v", gap)
		}
	}
	if _, ok := byDomain["firmware-flashing"]; !ok {
		t.Fatalf("missing firmware-flashing domain in coverage")
	}
}

func TestRegistryCapabilityPathsMatchCommandTree(t *testing.T) {
	root := (&app{}).rootCommand()
	commandPaths := make(map[string]struct{})
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Hidden {
			return
		}
		commandPaths[commandPath(cmd)] = struct{}{}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)

	for path := range capabilityMetadataRegistry() {
		if path == "betaflight-cli capabilities" {
			continue
		}
		if _, ok := commandPaths[path]; !ok {
			t.Fatalf("capability registry path %q is not present in the command tree", path)
		}
	}
}

func TestWriteOperationsDefaultAutoPortWhenNotProvided(t *testing.T) {
	var observed connection.Config
	env, err := runTestCommand(t, []string{"settings", "set", "small_angle", "5", "--apply", "--yes"}, func(_ context.Context, cfg connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		observed = cfg
		client, clientErr := connection.NewClient(fakefc.New(), time.Second)
		if clientErr != nil {
			return nil, connection.TargetInfo{}, clientErr
		}
		target, targetErr := client.Handshake(context.Background())
		if targetErr != nil {
			client.Close()
			return nil, connection.TargetInfo{}, targetErr
		}
		target.Port = "fake"
		return client, target, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	if !observed.AutoPort {
		t.Fatalf("expected AutoPort to default true for write commands: %#v", observed)
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
	env, err := runTestCommand(t, []string{"debug", "set-accelerometer-trim", "-12", "34"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	env, err := runTestCommand(t, []string{"debug", "set-accelerometer-trim", "-12", "34", "--yes"}, nil)
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

func TestTextSetRejectsReadOnlyFieldDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"text", "set", "build_key", "abc", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after read-only text field validation failure")
	}
}

func TestTextSetRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"text", "set", "craft_name", "Quad"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after text set confirmation failure")
	}
}

func TestTextSetWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"text", "set", "craft_name", "Quad", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	text := data["text"].(map[string]any)
	request := text["request"].(map[string]any)
	if request["key"] != "craft_name" || request["value"] != "Quad" || text["msp_name"] != "MSP2_SET_TEXT" || text["save_required"] != true {
		t.Fatalf("text set = %+v", text)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "text_set" || env.SideEffects[0].Command != "MSP2_SET_TEXT" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

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
	if err != nil {
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

func TestMSPRequestWithDecodeForGyroSensorsActive(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_GYRO_SENSOR_ACTIVE", "--decode"}, nil)
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
	if decoded["source"] != "MSP2_GYRO_SENSOR_ACTIVE" {
		t.Fatalf("decoded source = %v", decoded["source"])
	}
	if decoded["count"] != float64(2) {
		t.Fatalf("decoded count = %v", decoded["count"])
	}
	hardware := decoded["hardware"].([]any)
	if len(hardware) != 2 || hardware[0].(map[string]any)["hardware_id"] != float64(11) || hardware[1].(map[string]any)["name"] != "ICM45605" {
		t.Fatalf("decoded hardware = %+v", hardware)
	}
}

func TestMSPRequestWithDecodeForSensorConfigActive(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_SENSOR_CONFIG_ACTIVE", "--decode"}, nil)
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
	if len(decoded) != 6 || decoded[0] != float64(10) || decoded[5] != float64(15) {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForOSDWarnings(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_GET_OSD_WARNINGS", "--decode"}, nil)
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
	if decoded["display_attributes"] != float64(2) {
		t.Fatalf("decoded display_attributes = %v", decoded["display_attributes"])
	}
	if decoded["text"] != "LOW BATTERY" {
		t.Fatalf("decoded text = %v", decoded["text"])
	}
}

func TestMSPRequestWithDecodeForServoAndMotorOutputOrder(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_SERVO", "--decode"}, nil)
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
	servo := data["decoded"].([]any)
	if len(servo) != 6 || servo[0] != float64(1500) || servo[5] != float64(0) {
		t.Fatalf("decoded servo = %+v", servo)
	}

	env, err = runTestCommand(t, []string{"msp", "request", "MSP2_MOTOR_OUTPUT_REORDERING", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data = env.Data.(map[string]any)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	order := data["decoded"].([]any)
	if len(order) != 4 || order[0] != float64(0) || order[3] != float64(3) {
		t.Fatalf("order channels = %+v", order)
	}
}

func TestMSPRequestWithDecodeForRawIMU(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_RAW_IMU", "--decode"}, nil)
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
	imuRaw := decoded["accelerometer_raw"].([]any)
	if len(imuRaw) != 3 || imuRaw[0] != float64(2048) || imuRaw[1] != float64(-1024) {
		t.Fatalf("accelerometer_raw = %+v", imuRaw)
	}
	g := decoded["accelerometer_g"].([]any)
	if g[0] != float64(1) || g[1] != float64(-0.5) {
		t.Fatalf("accelerometer_g = %+v", g)
	}
}

func TestMSPRequestWithDecodeForDataflashErase(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_DATAFLASH_ERASE", "--decode"}, nil)
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
	if _, ok := data["decoded"].(map[string]any); !ok {
		t.Fatalf("decoded = %+v", data["decoded"])
	}
}

func TestMSPRequestWithDecodeForReboot(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_REBOOT", "--decode"}, nil)
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
	payload, ok := decoded["payload_bytes"].([]any)
	if !ok || len(payload) == 0 {
		t.Fatalf("decoded payload_bytes = %+v", decoded["payload_bytes"])
	}
	if payload[0] != float64(0) {
		t.Fatalf("decoded payload_bytes = %+v", payload)
	}
}

func TestMSPRequestWithDecodeForSensorConfig(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_SENSOR_CONFIG", "--decode"}, nil)
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
	sensorConfig := data["decoded"].([]any)
	if len(sensorConfig) != 5 {
		t.Fatalf("decoded sensor config = %+v", sensorConfig)
	}
	if sensorConfig[0].(map[string]any)["hardware_id"] != float64(1) || sensorConfig[0].(map[string]any)["available"] != true {
		t.Fatalf("sensor config[0] = %+v", sensorConfig[0])
	}
}

func TestMSPRequestWithDecodeForRXMapUsesNumberArray(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_RX_MAP", "--decode"}, nil)
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
	if decoded, ok := data["decoded"].([]any); ok {
		if len(decoded) != 4 || decoded[0] != float64(0) || decoded[1] != float64(1) {
			t.Fatalf("decoded = %+v", decoded)
		}
	} else {
		t.Fatalf("decoded type = %T, value = %+v", data["decoded"], data["decoded"])
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

func TestVTXSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "set-config", "5", "8", "2", "true", "5861", "2", "5662", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtx_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["band"] != float64(5) || config["pit_mode"] != true || config["frequency_mhz"] != float64(5861) || result["save_required"] != true {
		t.Fatalf("vtx_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_VTX_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "set-config", "5", "8", "2", "true", "5861", "2", "5662"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestVTXSetConfigValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "set-config", "5", "8", "2", "maybe", "5861", "2", "5662", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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

func TestModesSetRangeWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"modes", "set-range", "1", "52", "2", "16", "32", "1", "53", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["mode_range"].(map[string]any)
	row := result["range"].(map[string]any)
	rangeData := row["range"].(map[string]any)
	if row["index"] != float64(1) || row["mode_logic_name"] != "AND" || row["linked_to"] != float64(53) || rangeData["start_us"] != float64(1300) || result["save_required"] != true {
		t.Fatalf("mode_range = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_MODE_RANGE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestModesSetRangeRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"modes", "set-range", "1", "52", "2", "16", "32"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestModesSetRangeValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"modes", "set-range", "1", "52", "2", "16", "300", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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
	deadband := receiver["deadband"].(map[string]any)
	if deadband["deadband"] != float64(5) || deadband["yaw_deadband"] != float64(7) || deadband["deadband_3d_throttle"] != float64(50) {
		t.Fatalf("deadband = %+v", deadband)
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

func TestReceiverSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"receiver_config":{"serial_provider":2,"stick_max":2000,"stick_center":1500,"stick_min":1000,"spektrum_satellite_bind":0,"rx_min_usec":885,"rx_max_usec":2115,"airmode_activate_threshold":1350,"rx_spi_protocol":0,"rx_spi_id":0,"rx_spi_rf_channel_count":0,"fpv_cam_angle_degrees":10,"rc_smoothing_setpoint_cutoff":50,"rc_smoothing_throttle_cutoff":60,"rc_smoothing_auto_factor_throttle":70,"usb_cdc_hid_type":0,"rc_smoothing_auto_factor":80,"rc_smoothing":1,"elrs_uid":[1,2,3,4,5,6],"elrs_model_id":7}}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["receiver_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["serial_provider"] != float64(2) || config["stick_center"] != float64(1500) || result["save_required"] != true {
		t.Fatalf("receiver_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "receiver_config" || env.SideEffects[0].Command != "MSP_SET_RX_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetConfigJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"receiver_config":{"serial_provider":2,"stick_max":2000,"stick_center":1500,"stick_min":1000}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after receiver config confirmation failure")
	}
}

func TestReceiverSetRSSIChannelRejectsOutOfRangeDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "set-rssi-channel", "19", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after RSSI channel validation failure")
	}
}

func TestReceiverSetRSSIChannelRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "set-rssi-channel", "8"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after RSSI channel confirmation failure")
	}
}

func TestReceiverSetMapWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-map", "0", "1", "3", "2", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rc_map"].(map[string]any)
	names := result["names"].([]any)
	if result["save_required"] != true || len(names) != 4 || names[2] != "THROTTLE" || names[3] != "YAW" {
		t.Fatalf("rc_map = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_RX_MAP" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetMapRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-map", "0", "1", "3", "2"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestReceiverSetMapValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-map", "0", "1", "3", "300", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
	}
}

func TestReceiverSetMapRejectsDuplicateValuesBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-map", "0", "1", "1", "2", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
	}
}

func TestReceiverSetRSSIChannelWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-rssi-channel", "8", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rssi_channel"].(map[string]any)
	if result["channel"] != float64(8) || result["msp_name"] != "MSP_SET_RSSI_CONFIG" || result["save_required"] != true {
		t.Fatalf("rssi channel = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "rssi_channel" || env.SideEffects[0].Command != "MSP_SET_RSSI_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetDeadbandRejectsInvalidValueDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "set-deadband", "5", "300", "3", "50", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after RC deadband validation failure")
	}
}

func TestReceiverSetDeadbandRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "set-deadband", "5", "7", "3", "50"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after RC deadband confirmation failure")
	}
}

func TestReceiverSetDeadbandWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-deadband", "5", "7", "3", "50", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rc_deadband"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["deadband"] != float64(5) || config["yaw_deadband"] != float64(7) || config["deadband_3d_throttle"] != float64(50) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_RC_DEADBAND" || result["save_required"] != true {
		t.Fatalf("rc deadband result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "rc_deadband" || env.SideEffects[0].Command != "MSP_SET_RC_DEADBAND" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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

func TestGPSSetConfigRejectsInvalidBoolBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-config", "1", "0", "2", "1", "1", "1", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestGPSSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-config", "1", "0", "1", "1", "1", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestGPSSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-config", "1", "0", "1", "1", "1", "1", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["gps_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["provider"] != float64(1) || config["auto_config"] != true || config["ublox_use_galileo"] != true {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_GPS_CONFIG" || result["save_required"] != true {
		t.Fatalf("gps config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "gps_config" || env.SideEffects[0].Command != "MSP_SET_GPS_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestGPSSetRescueRejectsInvalidValueBeforeConnect(t *testing.T) {
	args := []string{"gps", "set-rescue", "3200", "100", "50", "1500", "1200", "1800", "1450", "1", "8", "500", "150", "2", "2", "30", "20", "--yes"}
	env, err := runTestCommand(t, args, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestGPSSetRescueRequiresConfirmationBeforeConnect(t *testing.T) {
	args := []string{"gps", "set-rescue", "3200", "100", "50", "1500", "1200", "1800", "1450", "1", "8", "500", "150", "1", "2", "30", "20"}
	env, err := runTestCommand(t, args, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestGPSSetRescueWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-rescue", "3200", "100", "50", "1500", "1200", "1800", "1450", "1", "8", "500", "150", "1", "2", "30", "20", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["gps_rescue"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["max_rescue_angle"] != float64(3200) || config["min_sats"] != float64(8) || config["initial_climb_m"] != float64(20) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_GPS_RESCUE" || result["save_required"] != true {
		t.Fatalf("gps rescue result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "gps_rescue" || env.SideEffects[0].Command != "MSP_SET_GPS_RESCUE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestGPSSetRescuePIDWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-rescue-pids", "80", "10", "5", "120", "20", "10", "45", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["gps_rescue_pids"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["altitude_p"] != float64(80) || config["velocity_p"] != float64(120) || config["yaw_p"] != float64(45) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_GPS_RESCUE_PIDS" || result["save_required"] != true {
		t.Fatalf("gps rescue pid result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "gps_rescue_pids" || env.SideEffects[0].Command != "MSP_SET_GPS_RESCUE_PIDS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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

func TestOSDSetCanvasWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-canvas", "53", "20", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_canvas"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["columns"] != float64(53) || config["rows"] != float64(20) || result["save_required"] != true || result["reboot_possible"] != true {
		t.Fatalf("osd_canvas = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_OSD_CANVAS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetCanvasRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-canvas", "53", "20"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestOSDSetCanvasValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-canvas", "300", "20", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
	}
}

func TestOSDSetPositionWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-position", "7", "2122", "1", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_position"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(7) || config["raw"] != float64(2122) || config["screen"] != float64(1) || result["save_required"] != true {
		t.Fatalf("osd_position = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_OSD_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetStatWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-stat", "3", "true", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_stat"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(3) || config["enabled"] != true || result["save_required"] != true {
		t.Fatalf("osd_stat = %+v", result)
	}
}

func TestOSDSetTimerWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-timer", "1", "1110", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_timer"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(1) || config["value"] != float64(1110) || result["save_required"] != true {
		t.Fatalf("osd_timer = %+v", result)
	}
}

func TestOSDSetPositionRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-position", "7", "2122"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestOSDSetStatValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-stat", "3", "maybe", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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

func TestSensorsCalibrationRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"sensors", "calibrate-accelerometer"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after calibration confirmation failure")
	}
}

func TestSensorsCalibrationWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "calibrate-magnetometer", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	calibration := data["sensor_calibration"].(map[string]any)
	if calibration["kind"] != "magnetometer" || calibration["msp_name"] != "MSP_MAG_CALIBRATION" || calibration["acknowledged"] != true {
		t.Fatalf("calibration = %+v", calibration)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "sensor_calibration" || env.SideEffects[0].Command != "MSP_MAG_CALIBRATION" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSensorsSetConfigRejectsInvalidValueBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-config", "1", "2", "3", "300", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-config", "1", "2", "3", "4"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-config", "1", "2", "3", "4", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["sensor_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["accelerometer"] != float64(1) || config["barometer"] != float64(2) || config["rangefinder"] != float64(4) {
		t.Fatalf("config = %+v", config)
	}
	hardware := result["hardware"].([]any)
	if len(hardware) != 4 || hardware[0].(map[string]any)["name"] != "accelerometer" || hardware[3].(map[string]any)["name"] != "rangefinder" {
		t.Fatalf("hardware = %+v", hardware)
	}
	if result["msp_name"] != "MSP_SET_SENSOR_CONFIG" || result["save_required"] != true {
		t.Fatalf("sensor config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "sensor_config" || env.SideEffects[0].Command != "MSP_SET_SENSOR_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSensorsSetAlignmentRejectsPartialCustomAlignmentBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-alignment", "2", "3", "-10", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetAlignmentRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-alignment", "2", "3", "-10", "20", "900"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetAlignmentWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-alignment", "2", "3", "-10", "20", "900", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["sensor_alignment"].(map[string]any)
	config := result["config"].(map[string]any)
	custom := config["mag_custom_alignment"].(map[string]any)
	if config["magnetometer_alignment"] != float64(2) || config["gyro_enabled_mask"] != float64(3) || custom["yaw"] != float64(900) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_SENSOR_ALIGNMENT" || result["save_required"] != true {
		t.Fatalf("sensor alignment result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "sensor_alignment" || env.SideEffects[0].Command != "MSP_SET_SENSOR_ALIGNMENT" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSensorsSetCompassDeclinationRejectsInvalidValueBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-compass-declination", "40000", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetCompassDeclinationRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-compass-declination", "123"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetCompassDeclinationWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-compass-declination", "-123", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["compass_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["declination_deci_degrees"] != float64(-123) || config["declination_degrees"] != -12.3 {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_COMPASS_CONFIG" || result["save_required"] != true {
		t.Fatalf("compass config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "compass_config" || env.SideEffects[0].Command != "MSP_SET_COMPASS_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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

func TestBeeperSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "set-config", "0x12", "3", "0x202", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["beeper_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["disabled_mask"] != float64(0x12) || config["dshot_beacon_disabled_mask"] != float64(0x202) || result["save_required"] != true {
		t.Fatalf("beeper_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_BEEPER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBeeperSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "set-config", "0x12", "3", "0x202"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBeeperSetConfigValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "set-config", "0x12", "300", "0x202", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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

func TestTransponderSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-config", "2", "18,52,86,120,154,188,222,240,66", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["transponder_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["provider"] != float64(2) || config["provider_name"] != "ARCITIMER" || config["data_hex"] != "123456789ABCDEF042" || result["save_required"] != true {
		t.Fatalf("transponder_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_TRANSPONDER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestTransponderSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-config", "2", "18,52,86"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestTransponderSetConfigValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-config", "0", "18", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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

func TestMixerSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"mixer_config":{"mode":3,"yaw_motors_reversed":true}}`
	env, err := runTestCommandWithInput(t, []string{"mixer", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["mixer_config"].(map[string]any)
	config := result["config"].(map[string]any)
	mode := result["mode"].(map[string]any)
	if config["mode"] != float64(3) || config["yaw_motors_reversed"] != true || mode["cli_name"] != "QUADX" || result["save_required"] != true {
		t.Fatalf("mixer_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "mixer_config" || env.SideEffects[0].Command != "MSP_SET_MIXER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMixerSetConfigJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"mixer_config":{"mode":3,"yaw_motors_reversed":true}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"mixer", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after mixer config confirmation failure")
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

func TestMotorsSetConfigRejectsInvalidBoolBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-config", "2000", "1000", "14", "2", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestMotorsSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-config", "2000", "1000", "14", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestMotorsSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-config", "2000", "1000", "14", "1", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["motor_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["max_throttle"] != float64(2000) || config["min_command"] != float64(1000) || config["use_dshot_telemetry"] != true {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_MOTOR_CONFIG" || result["save_required"] != true {
		t.Fatalf("motor config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_config" || env.SideEffects[0].Command != "MSP_SET_MOTOR_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMotorsSet3DConfigRejectsInvalidValueBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-3d-config", "1406", "70000", "1460", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestMotorsSet3DConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-3d-config", "1406", "1514", "1460"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestMotorsSet3DConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-3d-config", "1406", "1514", "1460", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["motor_3d_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["deadband_low"] != float64(1406) || config["deadband_high"] != float64(1514) || config["neutral"] != float64(1460) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_MOTOR_3D_CONFIG" || result["save_required"] != true {
		t.Fatalf("motor 3d config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_3d_config" || env.SideEffects[0].Command != "MSP_SET_MOTOR_3D_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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
	commandPreviews := plan["command_previews"].([]any)
	if len(commandPreviews) != 1 || commandPreviews[0] != "motor 2 1100" {
		t.Fatalf("command_previews = %+v", commandPreviews)
	}
	checks := plan["safety_checks"].([]any)
	if checks[0].(map[string]any)["passed"] != true || checks[1].(map[string]any)["passed"] != false {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestMotorsTestPlanAllMotors(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "test-plan", "--all", "--motor-count", "4", "--value", "1120", "--duration", "500ms", "--props-off"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)["motor_test_plan"].(map[string]any)
	if plan["all_motors"] != true || int(plan["motor_count"].(float64)) != 4 {
		t.Fatalf("plan = %+v", plan)
	}
	commandPreviews := plan["command_previews"].([]any)
	if len(commandPreviews) != 4 || commandPreviews[0] != "motor 0 1120" || commandPreviews[3] != "motor 3 1120" {
		t.Fatalf("command_previews = %+v", commandPreviews)
	}
	stopPreviews := plan["stop_command_previews"].([]any)
	if len(stopPreviews) != 4 || stopPreviews[0] != "motor 0 1000" || stopPreviews[3] != "motor 3 1000" {
		t.Fatalf("stop_command_previews = %+v", stopPreviews)
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
	commandPreviews := plan["command_previews"].([]any)
	if len(commandPreviews) != 1 || plan["command_preview"] != "motor 1 1050" {
		t.Fatalf("plan = %+v", plan)
	}
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
	if audit["start_command"] != "motor 1 1050" || audit["stop_command"] != "motor 1 1000" {
		t.Fatalf("audit = %+v", audit)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_output" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMotorsTestApplyAllMotors(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommand(t, []string{"motors", "test-apply", "--all", "--motor-count", "3", "--value", "1060", "--duration", "1ms", "--props-off", "--battery-aware", "--yes"}, func(ctx context.Context, cfg connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if plan["all_motors"] != true || plan["applied"] != true || plan["stopped"] != true {
		t.Fatalf("plan = %+v", plan)
	}
	commandPreviews := plan["command_previews"].([]any)
	if len(commandPreviews) != 3 {
		t.Fatalf("command_previews = %+v", commandPreviews)
	}
	stopPreviews := plan["stop_command_previews"].([]any)
	if len(stopPreviews) != 3 {
		t.Fatalf("stop_command_previews = %+v", stopPreviews)
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

func TestServosSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"servos", "set-config", "1", "1100", "1900", "1501", "-50", "2", "5", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["servo_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(1) || config["rate"] != float64(-50) || result["save_required"] != true {
		t.Fatalf("servo_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_SERVO_CONFIGURATION" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestServosSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"servos", "set-config", "1", "1100", "1900", "1501", "-50", "2", "5"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestServosSetConfigValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"servos", "set-config", "1", "1100", "1900", "1501", "-200", "2", "5", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
	}
}

func TestServosSetMixRuleWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"servos", "set-mix-rule", "2", "1", "3", "-25", "10", "5", "95", "4", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["servo_mix_rule"].(map[string]any)
	rule := result["rule"].(map[string]any)
	if rule["index"] != float64(2) || rule["rate"] != float64(-25) || rule["active"] != true || result["save_required"] != true {
		t.Fatalf("servo_mix_rule = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_SERVO_MIX_RULE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestServosSetMixRuleRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"servos", "set-mix-rule", "2", "1", "3", "-25", "10", "5", "95", "4"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
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

func TestFeaturesSetMaskWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "set-mask", "0x00040488", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["feature_mask"].(map[string]any)
	features := result["features"].(map[string]any)
	if features["mask"] != float64(0x00040488) || result["save_required"] != true {
		t.Fatalf("feature_mask = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_FEATURE_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFeaturesSetMaskRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "set-mask", "0x00040488"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFeaturesSetMaskValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "set-mask", "not-a-mask", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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
	env, err := runTestCommand(t, []string{"receiver", "rxfail", "2", "s", "1100", "--apply", "--yes"}, nil)
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

func TestReceiverSetRXFailWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-rxfail", "2", "2", "1100", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rx_fail"].(map[string]any)
	channel := result["channel"].(map[string]any)
	if channel["index"] != float64(2) || channel["mode_name"] != "SET" || channel["value"] != float64(1100) || result["save_required"] != true {
		t.Fatalf("rx_fail = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_RXFAIL_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetRXFailRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-rxfail", "2", "2", "1100"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestReceiverSetRXFailValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-rxfail", "4", "0", "1000", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFeatureEnableApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "enable", "gps", "--apply", "--yes"}, nil)
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
	env, err := runTestCommand(t, []string{"profiles", "battery-select", "1", "--apply", "--yes"}, nil)
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

func TestProfilesCopyRejectsUnsupportedKindDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"profiles", "copy", "battery", "0", "1", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after profile copy validation failure")
	}
}

func TestProfilesCopyRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"profiles", "copy", "pid", "0", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after profile copy confirmation failure")
	}
}

func TestProfilesCopyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"profiles", "copy", "rate", "1", "2", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	copyResult := data["profile_copy"].(map[string]any)
	request := copyResult["request"].(map[string]any)
	if request["kind"] != "rate" || request["source"] != float64(1) || request["destination"] != float64(2) {
		t.Fatalf("profile copy = %+v", copyResult)
	}
	if copyResult["msp_name"] != "MSP_COPY_PROFILE" || copyResult["save_required"] != true {
		t.Fatalf("profile copy = %+v", copyResult)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "profile_copy" || env.SideEffects[0].Command != "MSP_COPY_PROFILE" {
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
	if called {
		t.Fatal("connector was called after preset fetch confirmation failure")
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

func TestAdjustmentsSetRangeWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"adjustments", "set-range", "1", "0", "2", "4", "8", "33", "3", "1600", "50", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["adjustment_range"].(map[string]any)
	row := result["range"].(map[string]any)
	if row["index"] != float64(1) || row["range_start_us"] != float64(1000) || row["adjustment_function_name"] != "BATTERY_PROFILE" || result["save_required"] != true {
		t.Fatalf("adjustment_range = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_ADJUSTMENT_RANGE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestAdjustmentsSetRangeRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"adjustments", "set-range", "1", "0", "2", "4", "8", "33", "3", "1600", "50"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestAdjustmentsSetRangeValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"adjustments", "set-range", "1", "0", "2", "4", "8", "33", "3", "99999", "50", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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

func TestSerialApplyConfigJSONWithFakeFC(t *testing.T) {
	input := `{"ports":[{"identifier":51,"function_mask":66,"msp_baudrate_index":5,"gps_baudrate_index":4,"telemetry_baudrate_index":0,"blackbox_baudrate_index":0}]}`
	env, err := runTestCommandWithInput(t, []string{"serial", "apply-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["serial_config"].(map[string]any)
	ports := result["ports"].([]any)
	first := ports[0].(map[string]any)
	if result["save_required"] != true || first["identifier_name"] != "UART1" || first["function_mask"] != float64(66) {
		t.Fatalf("serial_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_CF_SERIAL_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSerialApplyConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `[{"identifier":51,"function_mask":66,"msp_baudrate_index":5,"gps_baudrate_index":4,"telemetry_baudrate_index":0,"blackbox_baudrate_index":0}]`
	env, err := runTestCommandWithInput(t, []string{"serial", "apply-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSerialApplyConfigJSONValidationBeforeConnect(t *testing.T) {
	input := `[{"identifier":51,"function_mask":70000,"msp_baudrate_index":5,"gps_baudrate_index":4,"telemetry_baudrate_index":0,"blackbox_baudrate_index":0}]`
	env, err := runTestCommandWithInput(t, []string{"serial", "apply-config-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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

func TestLEDSetValuesRejectsOutOfRangeDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"leds", "set-values", "300", "20", "120", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after LED values validation failure")
	}
}

func TestLEDSetValuesRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"leds", "set-values", "50", "20", "120"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after LED values confirmation failure")
	}
}

func TestLEDSetValuesWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"leds", "set-values", "50", "20", "120", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["led_values"].(map[string]any)
	values := result["values"].(map[string]any)
	if values["brightness"] != float64(50) || values["rainbow_delta"] != float64(20) || values["rainbow_freq"] != float64(120) {
		t.Fatalf("led values = %+v", result)
	}
	if result["msp_name"] != "MSP2_SET_LED_STRIP_CONFIG_VALUES" || result["save_required"] != true {
		t.Fatalf("led values = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "led_values" || env.SideEffects[0].Command != "MSP2_SET_LED_STRIP_CONFIG_VALUES" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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

func TestPIDSetGainsJSONWithFakeFC(t *testing.T) {
	input := `{"gains":[{"p":45,"i":80,"d":30},{"p":47,"i":84,"d":34},{"p":45,"i":80,"d":0},{"p":50,"i":50,"d":75},{"p":40,"i":0,"d":0}]}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-gains-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["pid_gains"].(map[string]any)
	gains := result["gains"].([]any)
	first := gains[0].(map[string]any)
	if first["name"] != "ROLL" || first["p"] != float64(45) || first["i"] != float64(80) || result["save_required"] != true {
		t.Fatalf("pid_gains = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_PID" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestPIDSetGainsJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `[{"p":45,"i":80,"d":30},{"p":47,"i":84,"d":34},{"p":45,"i":80,"d":0},{"p":50,"i":50,"d":75},{"p":40,"i":0,"d":0}]`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-gains-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestPIDSetGainsJSONValidationBeforeConnect(t *testing.T) {
	input := `[{"p":45,"i":80,"d":30}]`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-gains-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid JSON shape")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
	}
}

func TestPIDSetAdvancedJSONWithFakeFC(t *testing.T) {
	input := `{"pid_advanced":{"feedforward_transition":20,"rate_accel_limit":100,"yaw_rate_accel_limit":120,"level_angle_limit":55,"anti_gravity_gain":3500,"iterm_rotation":1,"iterm_relax":2,"iterm_relax_type":1,"throttle_boost":5,"acro_trainer_angle_limit":20,"feedforward_roll":120,"feedforward_pitch":125,"feedforward_yaw":120,"d_max_roll":40,"d_max_pitch":42,"d_max_gain":35,"d_max_advance":50,"integrated_yaw_relax":20,"iterm_relax_cutoff":15,"motor_output_limit":95,"idle_min_rpm":30,"feedforward_averaging":2,"feedforward_smooth_factor":10,"feedforward_boost":20,"feedforward_max_rate_limit":30,"feedforward_jitter_factor":90,"vbat_sag_compensation":5,"thrust_linearization":10,"tpa":{"mode":2,"rate":15,"breakpoint":1350}}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-advanced-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["pid_advanced"].(map[string]any)
	advanced := result["advanced"].(map[string]any)
	if advanced["anti_gravity_gain"] != float64(3500) || advanced["feedforward_pitch"] != float64(125) || result["save_required"] != true {
		t.Fatalf("pid_advanced = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_PID_ADVANCED" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestPIDSetAdvancedJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"pid_advanced":{"feedforward_averaging":2}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-advanced-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestPIDSetAdvancedJSONValidationBeforeConnect(t *testing.T) {
	input := `{"pid_advanced":{"feedforward_averaging":4}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-advanced-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid PID advanced config")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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

func TestRatesSetProfileJSONWithFakeFC(t *testing.T) {
	input := `{"rate_profile":{"axes":[{"axis":"roll","rc_rate":7,"expo":10,"rate":70,"rate_limit_dps":900},{"axis":"pitch","rc_rate":7,"expo":9,"rate":72,"rate_limit_dps":850},{"axis":"yaw","rc_rate":8,"expo":5,"rate":65,"rate_limit_dps":800}],"throttle":{"mid_percent":50,"expo_percent":20,"hover_percent":45,"limit_type":1,"limit_percent":80},"rates_type":3}}`
	env, err := runTestCommandWithInput(t, []string{"rates", "set-profile-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rate_profile"].(map[string]any)
	profile := result["rate_profile"].(map[string]any)
	axes := profile["axes"].([]any)
	roll := axes[0].(map[string]any)
	if roll["axis"] != "roll" || roll["rate_limit_dps"] != float64(900) || result["save_required"] != true {
		t.Fatalf("rate_profile = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_RC_TUNING" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRatesSetProfileJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"axes":[{"axis":"roll"},{"axis":"pitch"},{"axis":"yaw"}]}`
	env, err := runTestCommandWithInput(t, []string{"rates", "set-profile-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestRatesSetProfileJSONValidationBeforeConnect(t *testing.T) {
	input := `{"axes":[{"axis":"roll"},{"axis":"roll"},{"axis":"yaw"}]}`
	env, err := runTestCommandWithInput(t, []string{"rates", "set-profile-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid JSON shape")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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

func TestFiltersSetAdvancedJSONWithFakeFC(t *testing.T) {
	input := `{"advanced_config":{"gyro_sync_denom":1,"pid_process_denom":4,"use_unsynced_pwm":0,"motor_protocol":7,"motor_pwm_rate":480,"motor_idle":550,"gyro_use_32khz_deprecated":0,"motor_pwm_inversion":1,"gyro_to_use_deprecated":0,"gyro_high_fsr":1,"gyro_movement_calib_threshold":32,"gyro_calib_duration":125,"gyro_offset_yaw":10,"gyro_check_overflow":2,"debug_mode":5,"debug_mode_count":80}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-advanced-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["advanced_config"].(map[string]any)
	config := result["advanced_config"].(map[string]any)
	if config["pid_process_denom"] != float64(4) || config["motor_protocol"] != float64(7) || result["save_required"] != true {
		t.Fatalf("advanced_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_ADVANCED_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFiltersSetAdvancedJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"advanced_config":{"debug_mode":5,"debug_mode_count":80}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-advanced-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFiltersSetAdvancedJSONValidationBeforeConnect(t *testing.T) {
	input := `{"advanced_config":{"debug_mode":80,"debug_mode_count":80}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-advanced-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid advanced config")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFiltersSetFilterJSONWithFakeFC(t *testing.T) {
	input := `{"filter_config":{"legacy_gyro_lowpass_hz":90,"gyro_lpf1_static_hz":150,"gyro_lpf2_static_hz":500,"gyro_lpf1_type":0,"gyro_lpf2_type":2,"gyro_hardware_lpf":1,"gyro_32khz_hardware_lpf_deprecated":0,"gyro_soft_notch_hz_1":0,"gyro_soft_notch_cutoff_1":0,"gyro_soft_notch_hz_2":0,"gyro_soft_notch_cutoff_2":0,"dterm_lpf1_static_hz":100,"dterm_lpf2_static_hz":150,"dterm_lpf1_type":0,"dterm_lpf2_type":2,"dterm_notch_hz":0,"dterm_notch_cutoff":0,"yaw_lowpass_hz":0,"dynamic_lowpass":{"gyro_min_hz":100,"gyro_max_hz":400,"dterm_min_hz":80,"dterm_max_hz":200,"dterm_expo":5},"dynamic_notch":{"range_deprecated":0,"width_percent_deprecated":0,"q":300,"min_hz":100,"max_hz":600,"count":3},"rpm_filter":{"harmonics":3,"min_hz":100,"fade_range_hz":50,"q":500,"weights":[100,80,60]}}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-filter-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["filter_config"].(map[string]any)
	config := result["filter_config"].(map[string]any)
	if config["gyro_lpf1_static_hz"] != float64(150) || config["dterm_lpf2_type"] != float64(2) || result["save_required"] != true {
		t.Fatalf("filter_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_FILTER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFiltersSetFilterJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"filter_config":{"rpm_filter":{"weights":[100,80,60]}}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-filter-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFiltersSetFilterJSONValidationBeforeConnect(t *testing.T) {
	input := `{"filter_config":{"rpm_filter":{"fade_range_hz":1001,"weights":[100,80,60]}}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-filter-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid filter config")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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

func TestBatterySetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"battery_config":{"legacy_min_cell_voltage_v":3.3,"legacy_max_cell_voltage_v":4.3,"legacy_warning_cell_voltage_v":3.5,"capacity_mah":1300,"voltage_meter_source":1,"current_meter_source":1,"min_cell_voltage_v":3.3,"max_cell_voltage_v":4.35,"warning_cell_voltage_v":3.5}}`
	env, err := runTestCommandWithInput(t, []string{"battery", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["battery_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["capacity_mah"] != float64(1300) || config["voltage_meter_source_name"] != "ADC" || result["save_required"] != true {
		t.Fatalf("battery_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "battery_config" || env.SideEffects[0].Command != "MSP_SET_BATTERY_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBatterySetConfigJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"battery_config":{"capacity_mah":1300,"voltage_meter_source":1,"current_meter_source":1,"min_cell_voltage_v":3.3,"max_cell_voltage_v":4.35,"warning_cell_voltage_v":3.5}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"battery", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after battery config confirmation failure")
	}
}

func TestBatterySetConfigJSONValidationBeforeConnect(t *testing.T) {
	input := `{"battery_config":{"capacity_mah":1300,"min_cell_voltage_v":3.6,"max_cell_voltage_v":4.35,"warning_cell_voltage_v":3.5}}`
	env, err := runTestCommandWithInput(t, []string{"battery", "set-config-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid battery config")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBatterySetVoltageMeterRejectsOutOfRangeDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"battery", "set-voltage-meter", "10", "300", "10", "1", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after voltage meter validation failure")
	}
}

func TestBatterySetVoltageMeterRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"battery", "set-voltage-meter", "10", "110", "10", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after voltage meter confirmation failure")
	}
}

func TestBatterySetVoltageMeterWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"battery", "set-voltage-meter", "10", "110", "10", "1", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["voltage_meter_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["id"] != float64(10) || config["vbat_scale"] != float64(110) || config["vbat_res_div_val"] != float64(10) || config["vbat_res_div_multiplier"] != float64(1) {
		t.Fatalf("voltage meter config = %+v", result)
	}
	if result["msp_name"] != "MSP_SET_VOLTAGE_METER_CONFIG" || result["save_required"] != true {
		t.Fatalf("voltage meter config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "voltage_meter_config" || env.SideEffects[0].Command != "MSP_SET_VOLTAGE_METER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBatterySetCurrentMeterRejectsOutOfRangeDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"battery", "set-current-meter", "10", "40000", "-10", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after current meter validation failure")
	}
}

func TestBatterySetCurrentMeterRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"battery", "set-current-meter", "10", "400", "-10"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after current meter confirmation failure")
	}
}

func TestBatterySetCurrentMeterWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"battery", "set-current-meter", "10", "400", "-10", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["current_meter_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["id"] != float64(10) || config["scale"] != float64(400) || config["offset"] != float64(-10) {
		t.Fatalf("current meter config = %+v", result)
	}
	if result["msp_name"] != "MSP_SET_CURRENT_METER_CONFIG" || result["save_required"] != true {
		t.Fatalf("current meter config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "current_meter_config" || env.SideEffects[0].Command != "MSP_SET_CURRENT_METER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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

func TestFailsafeSetBoardAlignmentRequiresValidInt16(t *testing.T) {
	env, err := runTestCommand(t, []string{"failsafe", "set-board-alignment", "0", "40000", "0", "--yes"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestFailsafeSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"failsafe_config":{"delay_tenths_s":15,"landing_time_s":60,"throttle":1000,"switch_mode":2,"throttle_low_delay_tenths_s":100,"procedure":1}}`
	env, err := runTestCommandWithInput(t, []string{"failsafe", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["failsafe_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["throttle"] != float64(1000) || config["switch_mode"] != float64(2) || result["save_required"] != true {
		t.Fatalf("failsafe_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "failsafe_config" || env.SideEffects[0].Command != "MSP_SET_FAILSAFE_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFailsafeSetConfigJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"failsafe_config":{"delay_tenths_s":15,"landing_time_s":60,"throttle":1000,"switch_mode":2,"throttle_low_delay_tenths_s":100,"procedure":1}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"failsafe", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after failsafe config confirmation failure")
	}
}

func TestFailsafeSetBoardAlignmentRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"failsafe", "set-board-alignment", "-2", "3", "90"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after board alignment confirmation failure")
	}
}

func TestFailsafeSetBoardAlignmentWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"failsafe", "set-board-alignment", "-2", "3", "90", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["board_alignment"].(map[string]any)
	alignment := result["board_alignment"].(map[string]any)
	if alignment["roll_degrees"] != float64(-2) || alignment["pitch_degrees"] != float64(3) || alignment["yaw_degrees"] != float64(90) {
		t.Fatalf("board alignment = %+v", result)
	}
	if result["msp_name"] != "MSP_SET_BOARD_ALIGNMENT_CONFIG" || result["save_required"] != true {
		t.Fatalf("board alignment = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "board_alignment" || env.SideEffects[0].Command != "MSP_SET_BOARD_ALIGNMENT_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
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

func TestVTXTableSetBandWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "set-band", "1", "raceband", "r", "true", "5658", "5695", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtxtable_band"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["name"] != "RACEBAND" || config["letter"] != "R" || config["factory"] != true || result["save_required"] != true {
		t.Fatalf("vtxtable_band = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_VTXTABLE_BAND" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXTableSetPowerWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "set-power", "2", "200", "200", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtxtable_power"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["level"] != float64(2) || config["value"] != float64(200) || config["label"] != "200" || result["save_required"] != true {
		t.Fatalf("vtxtable_power = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_VTXTABLE_POWERLEVEL" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXTableSetBandRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "set-band", "1", "raceband", "r", "true", "5658"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestVTXTableSetPowerValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "set-power", "2", "200", "TOOLONG", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_failed" {
		t.Fatalf("env = %+v", env)
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
	env, err := runTestCommand(t, []string{"rxrange", "set", "0", "1000", "2000", "--apply", "--yes"}, nil)
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
		if tt.args[0] == "vtx" {
			if _, ok := view["vtx"]; !ok {
				t.Fatalf("%v view = %+v", tt.args, view)
			}
			table, ok := view["vtx_table"].([]any)
			if !ok || len(table) == 0 {
				t.Fatalf("%v vtx_table = %+v", tt.args, view["vtx_table"])
			}
		}
		if tt.args[0] == "osd" {
			if _, ok := view["osd"]; !ok {
				t.Fatalf("%v view = %+v", tt.args, view)
			}
			osdLines, ok := view["lines"].([]any)
			if !ok || len(osdLines) == 0 {
				t.Fatalf("%v osd lines = %+v", tt.args, view["lines"])
			}
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
	env, err := runTestCommand(t, []string{"pid", "set", "p_roll", "46", "--apply", "--yes"}, nil)
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
